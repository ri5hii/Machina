package main

import (
	"fmt"
	"strings"
)

type jobSpec struct {
	Profile          string
	JobType          string
	FileName         string
	TypeName         string
	InputTypeName    string
	ResultTypeName   string
	ConstructorName  string
	DefaultChunkSize int
}

// newJobSpec derives generated type names and filenames from a profile and job name.
func newJobSpec(profile, jobName string) jobSpec {
	typeName := toPascalCase(jobName)
	return jobSpec{
		Profile:          profile,
		JobType:          jobName,
		FileName:         jobName,
		TypeName:         typeName + "Job",
		InputTypeName:    typeName + "Input",
		ResultTypeName:   typeName + "Result",
		ConstructorName:  typeName + "PayloadConstructor",
		DefaultChunkSize: 4,
	}
}

// normalizeJobName converts user input into the snake_case job names used by the registry.
func normalizeJobName(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case r == '-' || r == '_' || r == ' ':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// toPascalCase converts a snake_case job name into exported Go type names.
func toPascalCase(name string) string {
	parts := strings.Split(name, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			b.WriteString(part[1:])
		}
	}
	return b.String()
}

// buildJobTemplate selects the scaffold for the requested job profile.
func buildJobTemplate(spec jobSpec) string {
	if spec.Profile == "singleRun" {
		return buildParallelTemplate(spec)
	}
	return buildBatchTemplate(spec)
}

// buildParallelTemplate renders the single-run job scaffold used by register.
func buildParallelTemplate(spec jobSpec) string {
	return fmt.Sprintf(`package jobs

import (
	"context"
	"fmt"
)

// %s is a scaffold for a single-run job.
//
// Replace the placeholder fields and logic with your actual domain model.
// Typical setup:
// 1. Define the payload fields your API should accept in %s.
// 2. Validate and default that input in Validate.
// 3. Put the real work in Run.
// 4. Shape the response in %s.
type %s struct {
	// TODO: Replace these example fields with the real payload for your job.
	// Keep the json tags in sync with the API payload you want to accept.
	InputPath string `+"`json:\"input_path\"`"+`
	OutputPath string `+"`json:\"output_path\"`"+`
}

type %s struct {
	// TODO: Replace this with the data you want to return when the job finishes.
	Message string
}

type %s struct {
	Input %s
}

func (j *%s) JobType() string { return %q }

func (j *%s) Validate() error {
	// TODO: Validate required input and set any defaults your job needs.
	if j.Input.InputPath == "" {
		return fmt.Errorf("%s: input_path is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("%s: output_path is required")
	}
	return nil
}

func (j *%s) Run(ctx context.Context) (any, error) {
	if err := j.Validate(); err != nil {
		return nil, err
	}

	// TODO: Replace this stub with the real job logic.
	// This profile is best when the work is a single unit and does not need
	// item-by-item batching.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return %s{
		Message: "replace with your job result",
	}, nil
}
`, spec.TypeName, spec.InputTypeName, spec.ResultTypeName, spec.InputTypeName, spec.ResultTypeName, spec.TypeName, spec.InputTypeName, spec.TypeName, spec.JobType, spec.TypeName, spec.JobType, spec.JobType, spec.TypeName, spec.ResultTypeName)
}

// buildBatchTemplate renders the batch job scaffold used by register.
func buildBatchTemplate(spec jobSpec) string {
	return fmt.Sprintf(`package jobs

import (
	"context"
	"fmt"
)

// %s is a scaffold for a batch job.
//
// Use this profile when your work can be split into many independent items.
// Typical setup:
// 1. Define the payload fields your API should accept in %s.
// 2. Discover work items in Scan.
// 3. Process one chunk at a time in RunBatch.
// 4. Merge partial results in Aggregate.
type %s struct {
	// TODO: Replace these example fields with the real payload for your job.
	// Keep the json tags in sync with the API payload you want to accept.
	InputPath string `+"`json:\"input_path\"`"+`
	OutputPath string `+"`json:\"output_path\"`"+`
}

type %s struct {
	// TODO: Replace this with the summary your batch job should return.
	TotalItems int
	OutputPath string
}

type %s struct {
	Input %s
}

func (j *%s) JobType() string { return %q }

func (j *%s) Validate() error {
	// TODO: Validate required input and set any defaults your job needs.
	if j.Input.InputPath == "" {
		return fmt.Errorf("%s: input_path is required")
	}
	if j.Input.OutputPath == "" {
		return fmt.Errorf("%s: output_path is required")
	}
	return nil
}

func (j *%s) ChunkSize() int { return %d }

func (j *%s) Scan() ([]Item, error) {
	if err := j.Validate(); err != nil {
		return nil, err
	}

	// TODO: Discover the work items and return them as []Item.
	// Each element returned here will later be passed into RunBatch.
	return nil, nil
}

func (j *%s) RunBatch(ctx context.Context, batch []Item) (any, error) {
	// TODO: Process one chunk of items here.
	// Keep the return value small and focused on partial results that Aggregate
	// can merge later.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return %s{
		TotalItems: len(batch),
		OutputPath: j.Input.OutputPath,
	}, nil
}

func (j *%s) Aggregate(results []any) (any, error) {
	final := %s{
		OutputPath: j.Input.OutputPath,
	}

	for _, partial := range results {
		result := partial.(%s)
		final.TotalItems += result.TotalItems
	}

	// TODO: Merge partial batch results into the final response.
	return final, nil
}
`, spec.TypeName, spec.InputTypeName, spec.InputTypeName, spec.ResultTypeName, spec.TypeName, spec.InputTypeName, spec.TypeName, spec.JobType, spec.TypeName, spec.JobType, spec.JobType, spec.TypeName, spec.DefaultChunkSize, spec.TypeName, spec.TypeName, spec.ResultTypeName, spec.TypeName, spec.ResultTypeName, spec.ResultTypeName)
}
