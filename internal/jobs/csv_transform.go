package jobs

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type CSVTransformInput struct {
	InputPath       string `json:"input_path"`
	OutputPath      string `json:"output_path"`
	TransformType   string `json:"transform_type"`
	HasHeader       bool   `json:"has_header"`
	ColumnSeparator string `json:"column_separator"`
}

type CSVTransformResult struct {
	TotalRows     int
	Succeeded     int
	Failed        int
	FailedRows    []int
	OutputPath    string
	TransformType string
}

type csvRow struct {
	index  int
	fields []string
}

type csvBatchPartial struct {
	result CSVTransformResult
	rows   map[int][]string
}

type CSVTransformJob struct {
	Input      CSVTransformInput
	header     []string
	totalItems int
	separator  rune
}

// JobType reports the runtime registry key for this job.
func (j *CSVTransformJob) JobType() string { return "csv_transform" }

// Validate checks the CSV transform input before execution starts.
func (j *CSVTransformJob) Validate() error {
	if j.Input.InputPath == "" {
		return fmt.Errorf("csv_transform: input_path is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("csv_transform: output_path is required")
	}
	if j.Input.InputPath == j.Input.OutputPath {
		return fmt.Errorf("csv_transform: input_path and output_path must be different")
	}

	if j.Input.TransformType == "" {
		j.Input.TransformType = "trim"
	}
	switch j.Input.TransformType {
	case "uppercase", "lowercase", "trim":
	default:
		return fmt.Errorf("csv_transform: unknown transform_type %q (allowed: uppercase, lowercase, trim)", j.Input.TransformType)
	}

	sep, err := parseCSVSeparator(j.Input.ColumnSeparator)
	if err != nil {
		return fmt.Errorf("csv_transform: invalid column_separator: %w", err)
	}
	j.separator = sep

	return nil
}

// MaxRetries reports the retry policy for this job type.
func (j *CSVTransformJob) MaxRetries() int { return 1 }

// ChunkSize controls how many CSV rows are processed per batch.
func (j *CSVTransformJob) ChunkSize() int { return 4 }

// Scan reads the input CSV and turns rows into batch work items.
func (j *CSVTransformJob) Scan() ([]Item, error) {
	if j.Input.TransformType == "" {
		j.Input.TransformType = "trim"
	}

	if j.separator == 0 {
		sep, err := parseCSVSeparator(j.Input.ColumnSeparator)
		if err != nil {
			sep = ','
		}
		j.separator = sep
	}

	rows, header, err := readCSV(j.Input.InputPath, j.separator, j.Input.HasHeader)
	if err != nil {
		return nil, fmt.Errorf("csv_transform: failed to read %q: %w", j.Input.InputPath, err)
	}
	j.header = header
	j.totalItems = len(rows)
	items := make([]Item, len(rows))
	for i, row := range rows {
		items[i] = csvRow{index: i, fields: row}
	}
	return items, nil
}

// RunBatch applies the configured transform to one chunk of CSV rows.
func (j *CSVTransformJob) RunBatch(ctx context.Context, batch []Item) (any, error) {
	partial := csvBatchPartial{
		result: CSVTransformResult{
			TotalRows:     len(batch),
			OutputPath:    j.Input.OutputPath,
			TransformType: j.Input.TransformType,
		},
		rows: make(map[int][]string, len(batch)),
	}

	for _, item := range batch {
		if ctx.Err() != nil {
			return partial, fmt.Errorf(
				"csv_transform: cancelled after processing %d/%d rows: %w",
				partial.result.Succeeded+partial.result.Failed,
				partial.result.TotalRows,
				ctx.Err(),
			)
		}

		row := item.(csvRow)
		out, err := transformRow(row.fields, j.Input.TransformType)
		if err != nil {
			partial.result.Failed++
			partial.result.FailedRows = append(partial.result.FailedRows, row.index)
			continue
		}

		partial.result.Succeeded++
		partial.rows[row.index] = out
	}

	return partial, nil
}

// Aggregate merges transformed row batches and writes the final output CSV.
func (j *CSVTransformJob) Aggregate(partials []any) (any, error) {
	final := CSVTransformResult{
		OutputPath:    j.Input.OutputPath,
		TransformType: j.Input.TransformType,
	}

	transformed := make([][]string, j.totalItems)

	for _, p := range partials {
		bp := p.(csvBatchPartial)
		final.TotalRows += bp.result.TotalRows
		final.Succeeded += bp.result.Succeeded
		final.Failed += bp.result.Failed
		final.FailedRows = append(final.FailedRows, bp.result.FailedRows...)
		for idx, row := range bp.rows {
			transformed[idx] = row
		}
	}

	if err := writeCSV(j.Input.OutputPath, j.header, transformed, j.separator); err != nil {
		return final, fmt.Errorf("csv_transform: failed to write output %q: %w", j.Input.OutputPath, err)
	}

	return final, nil
}

// parseCSVSeparator converts the configured separator string into a CSV delimiter rune.
func parseCSVSeparator(raw string) (rune, error) {
	if raw == "" {
		return ',', nil
	}

	switch raw {
	case `\t`, "tab", "TAB":
		return '\t', nil
	case `\n`:
		return '\n', nil
	case `\r`:
		return '\r', nil
	}

	r := []rune(raw)
	if len(r) != 1 {
		return 0, fmt.Errorf("separator must be a single character, got %q", raw)
	}

	sep := r[0]
	if sep == '"' {
		return 0, fmt.Errorf(`separator cannot be '"'`)
	}
	return sep, nil
}

// transformRow applies one supported text transform to every field in a row.
func transformRow(fields []string, transformType string) ([]string, error) {
	out := make([]string, len(fields))
	for i, field := range fields {
		switch transformType {
		case "uppercase":
			out[i] = strings.ToUpper(field)
		case "lowercase":
			out[i] = strings.ToLower(field)
		case "trim":
			out[i] = strings.TrimSpace(field)
		default:
			return nil, fmt.Errorf("unknown transform type: %q", transformType)
		}
	}
	return out, nil
}

// readCSV loads an input CSV file and optionally separates the header row.
func readCSV(inputPath string, separator rune, hasHeader bool) ([][]string, []string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open %q: %w", inputPath, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = separator
	r.TrimLeadingSpace = false

	allRows, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CSV %q: %w", inputPath, err)
	}

	if len(allRows) == 0 {
		return nil, nil, nil
	}

	var header []string
	dataRows := allRows
	if hasHeader {
		header = allRows[0]
		dataRows = allRows[1:]
	}

	return dataRows, header, nil
}

// writeCSV writes the transformed header and rows to the target output path.
func writeCSV(outputPath string, header []string, rows [][]string, separator rune) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", outputPath, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	w.Comma = separator

	if header != nil {
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	for _, row := range rows {
		if row == nil {
			continue
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	w.Flush()
	return w.Error()
}
