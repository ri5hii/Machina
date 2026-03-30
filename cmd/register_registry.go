package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func registeredJobTypes() ([]string, error) {
	registryPath := filepath.Join("internal", "registry", "payloadConstructor.go")
	content, err := os.ReadFile(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry file: %w", err)
	}

	var profiles []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `reg.Register("`) {
			continue
		}

		line = strings.TrimPrefix(line, `reg.Register("`)
		idx := strings.Index(line, `"`)
		if idx == -1 {
			continue
		}
		profiles = append(profiles, line[:idx])
	}

	sort.Strings(profiles)
	return profiles, nil
}

func appendRegistryConstructor(spec jobSpec) error {
	registryPath := filepath.Join("internal", "registry", "payloadConstructor.go")
	content, err := os.ReadFile(registryPath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	if bytes.Contains(content, []byte(`reg.Register("`+spec.JobType+`", `)) {
		return fmt.Errorf("job %q already registered in registry", spec.JobType)
	}

	constructor := fmt.Sprintf(`
func %s(payload map[string]any) (jobs.JobRunType, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to encode payload: %%w", err)
	}

	var input jobs.%s
	err = json.Unmarshal(b, &input)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to decode payload: %%w", err)
	}

	return &jobs.%s{Input: input}, nil
}
`, spec.ConstructorName, spec.JobType, spec.InputTypeName, spec.JobType, spec.TypeName)

	registerLine := fmt.Sprintf("\n\treg.Register(%q, %s)", spec.JobType, spec.ConstructorName)
	oldRegisterBlock := "func (reg *Registry) RegisterJobs() {"
	start := strings.Index(string(content), oldRegisterBlock)
	if start == -1 {
		return fmt.Errorf("failed to locate RegisterJob function in registry")
	}

	before := strings.TrimRight(string(content[:start]), "\n")
	registerBlock := string(content[start:])
	registerEnd := strings.LastIndex(registerBlock, "}")
	if registerEnd == -1 {
		return fmt.Errorf("failed to locate end of RegisterJob function")
	}

	registerBody := registerBlock[:registerEnd]
	registerTail := registerBlock[registerEnd:]
	updated := before + "\n\n" + strings.TrimSpace(constructor) + "\n\n" + registerBody + registerLine + registerTail

	if err := os.WriteFile(registryPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}
	return nil
}

func removeRegistryConstructor(spec jobSpec) error {
	registryPath := filepath.Join("internal", "registry", "payloadConstructor.go")
	content, err := os.ReadFile(registryPath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	updated := string(content)
	registerLine := fmt.Sprintf("\n\treg.Register(%q, %s)", spec.JobType, spec.ConstructorName)
	if !strings.Contains(updated, registerLine) {
		return fmt.Errorf("job %q is not registered in registry", spec.JobType)
	}
	updated = strings.Replace(updated, registerLine, "", 1)

	funcSignature := fmt.Sprintf("func %s(payload map[string]any) (jobs.JobRunType, error) {", spec.ConstructorName)
	start := strings.Index(updated, funcSignature)
	if start == -1 {
		return fmt.Errorf("constructor for %q not found in registry", spec.JobType)
	}

	end, err := findFunctionEnd(updated, start)
	if err != nil {
		return err
	}

	updated = strings.TrimRight(updated[:start], "\n") + "\n\n" + strings.TrimLeft(updated[end:], "\n")
	if err := os.WriteFile(registryPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}
	return nil
}

func findFunctionEnd(content string, start int) (int, error) {
	openBrace := strings.Index(content[start:], "{")
	if openBrace == -1 {
		return 0, fmt.Errorf("failed to locate function body")
	}
	openBrace += start

	depth := 0
	for i := openBrace; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}

	return 0, fmt.Errorf("failed to locate end of function body")
}

func gofmtFiles(paths ...string) error {
	args := append([]string{"-w"}, paths...)
	cmd := exec.Command("gofmt", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		sort.Strings(paths)
		return fmt.Errorf("gofmt failed for %v: %w", paths, err)
	}
	return nil
}
