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
	ColumnSeparator rune   `json:"column_separator"`
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
}

func (j *CSVTransformJob) JobType() string { return "csv_transform" }

func (j *CSVTransformJob) Validate() error {
	if j.Input.InputPath == "" {
		return fmt.Errorf("csv_transform: InputPath is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("csv_transform: OutputPath is required")
	}
	if j.Input.InputPath == j.Input.OutputPath {
		return fmt.Errorf("csv_transform: InputPath and OutputPath must be different")
	}
	validTransforms := map[string]bool{"uppercase": true, "lowercase": true, "trim": true}
	if j.Input.TransformType == "" {
		j.Input.TransformType = "trim"
	}
	if !validTransforms[j.Input.TransformType] {
		return fmt.Errorf("csv_transform: unknown TransformType %q — must be one of: uppercase, lowercase, trim",
			j.Input.TransformType)
	}
	if j.Input.ColumnSeparator == 0 {
		j.Input.ColumnSeparator = ','
	}
	return nil
}

func (j *CSVTransformJob) MaxRetries() int { return 1 }

func (j *CSVTransformJob) ChunkSize() int { return 4 }

func (j *CSVTransformJob) Scan() ([]Item, error) {
	rows, header, err := readCSV(j.Input.InputPath, j.Input.ColumnSeparator, j.Input.HasHeader)
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
			return partial, fmt.Errorf("csv_transform: cancelled after processing %d/%d rows: %w",
				partial.result.Succeeded+partial.result.Failed, partial.result.TotalRows, ctx.Err())
		}

		row := item.(csvRow)
		out, err := transformRow(row.fields, j.Input.TransformType)
		if err != nil {
			partial.result.Failed++
			partial.result.FailedRows = append(partial.result.FailedRows, row.index)
		} else {
			partial.result.Succeeded++
			partial.rows[row.index] = out
		}
	}

	return partial, nil
}

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

	if err := writeCSV(j.Input.OutputPath, j.header, transformed, j.Input.ColumnSeparator); err != nil {
		return final, fmt.Errorf("csv_transform: failed to write output %q: %w", j.Input.OutputPath, err)
	}

	return final, nil
}

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
