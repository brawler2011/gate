package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var embeddedSchema []byte

var (
	h2ContextRegex  = regexp.MustCompile(`(?m)^##\s+(Context|Summary)\s*$`)
	h2CriteriaRegex = regexp.MustCompile(`(?m)^##\s+Acceptance Criteria\s*$`)
	h2HeadingRegex  = regexp.MustCompile(`(?m)^##\s+`)
	checklistRegex  = regexp.MustCompile(`(?m)^\s*-\s*\[[ xX]\]\s+\S+`)
	uncheckedRegex  = regexp.MustCompile(`(?m)^\s*-\s*\[ \]\s+\S+`)
)

// Validator validates task markdown files.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator initializes a validator with the embedded JSON schema.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(embeddedSchema))
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded schema JSON: %w", err)
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("failed to register embedded schema: %w", err)
	}

	sch, err := c.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile embedded schema: %w", err)
	}

	return &Validator{schema: sch}, nil
}

// TaskFrontmatter represents the parsed frontmatter fields.
type TaskFrontmatter struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Status      string   `yaml:"status"`
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Priority    string   `yaml:"priority"`
	CreatedAt   any      `yaml:"created_at"`
	Assignee    string   `yaml:"assignee"`
	Tags        []string `yaml:"tags"`
}

// IsTemplateFile returns true if the filename represents a template.
func IsTemplateFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return base == "template.md" || strings.HasPrefix(base, "template-")
}

// ValidateContent checks whether task markdown content complies with the specification.
func (v *Validator) ValidateContent(filePath string, content []byte) error {
	if IsTemplateFile(filePath) {
		return nil
	}

	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	frontmatterBytes, bodyBytes, err := extractFrontmatter(content)
	if err != nil {
		return fmt.Errorf("%s: %w", filePath, err)
	}

	var rawMap map[string]any
	if err := yaml.Unmarshal(frontmatterBytes, &rawMap); err != nil {
		return fmt.Errorf("%s: invalid YAML frontmatter: %w", filePath, err)
	}

	// Normalize date if yaml parser parsed it as time.Time
	if val, ok := rawMap["created_at"].(time.Time); ok {
		rawMap["created_at"] = val.Format("2006-01-02")
	}

	// Validate against JSON schema
	jsonBytes, err := json.Marshal(rawMap)
	if err != nil {
		return fmt.Errorf("%s: failed to convert frontmatter to JSON: %w", filePath, err)
	}

	var jsonVal any
	if err := json.Unmarshal(jsonBytes, &jsonVal); err != nil {
		return fmt.Errorf("%s: failed to decode JSON representation: %w", filePath, err)
	}

	if err := v.schema.Validate(jsonVal); err != nil {
		return fmt.Errorf("%s: schema validation failed:\n  %s", filePath, formatSchemaError(err))
	}

	// Validate filename vs ID matching
	taskID, _ := rawMap["id"].(string)
	if err := validateFilename(filePath, taskID); err != nil {
		return fmt.Errorf("%s: %w", filePath, err)
	}

	// Validate Markdown body structure
	taskStatus, _ := rawMap["status"].(string)
	if err := validateBody(bodyBytes, taskStatus); err != nil {
		return fmt.Errorf("%s: body structure validation failed: %w", filePath, err)
	}

	return nil
}

func extractFrontmatter(content []byte) (frontmatter, body []byte, err error) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil, fmt.Errorf("task file must start with frontmatter delimiter '---'")
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, nil, fmt.Errorf("unclosed frontmatter delimiter '---'")
	}

	fmLines := lines[1:endIdx]
	bodyLines := lines[endIdx+1:]

	return []byte(strings.Join(fmLines, "\n")), []byte(strings.Join(bodyLines, "\n")), nil
}

func validateFilename(filePath string, taskID string) error {
	base := filepath.Base(filePath)
	if !strings.HasSuffix(base, ".md") {
		return fmt.Errorf("file must have .md extension, got %q", base)
	}

	// Expected formats: TASK-001.md or TASK-001-slug.md or TASK-001_slug.md
	if base != taskID+".md" && !strings.HasPrefix(base, taskID+"-") && !strings.HasPrefix(base, taskID+"_") {
		return fmt.Errorf("filename %q does not match task ID %q (expected filename to start with %q)", base, taskID, taskID)
	}

	return nil
}

func validateBody(bodyBytes []byte, taskStatus string) error {
	body := string(bodyBytes)

	// Check ## Context or ## Summary
	contextLoc := h2ContextRegex.FindStringIndex(body)
	if contextLoc == nil {
		return fmt.Errorf("missing required heading '## Context' or '## Summary'")
	}

	// Check ## Acceptance Criteria
	criteriaLoc := h2CriteriaRegex.FindStringIndex(body)
	if criteriaLoc == nil {
		return fmt.Errorf("missing required heading '## Acceptance Criteria'")
	}

	// Verify Context is not empty (has content before next heading or EOF)
	contextStart := contextLoc[1]
	contextEnd := len(body)
	if criteriaLoc[0] > contextStart {
		contextEnd = criteriaLoc[0]
	}
	contextContent := strings.TrimSpace(body[contextStart:contextEnd])
	if contextContent == "" {
		return fmt.Errorf("section '## Context' (or '## Summary') must not be empty")
	}

	// Extract Acceptance Criteria section up to next H2 heading or EOF
	criteriaStart := criteriaLoc[1]
	criteriaEnd := len(body)
	if loc := h2HeadingRegex.FindStringIndex(body[criteriaStart:]); loc != nil {
		criteriaEnd = criteriaStart + loc[0]
	}
	criteriaContent := body[criteriaStart:criteriaEnd]

	// Check for at least one checklist item in Acceptance Criteria
	if !checklistRegex.MatchString(criteriaContent) {
		return fmt.Errorf("section '## Acceptance Criteria' must contain at least one checklist item (e.g. '- [ ] ...')")
	}

	// If status is 'done', ensure all criteria items are completed
	if taskStatus == "done" {
		unchecked := uncheckedRegex.FindAllString(criteriaContent, -1)
		if len(unchecked) > 0 {
			return fmt.Errorf("task status is 'done', but contains %d unchecked acceptance criteria: all items must be marked as completed '- [x]'", len(unchecked))
		}
	}

	return nil
}

func formatSchemaError(err error) string {
	if validationErr, ok := err.(*jsonschema.ValidationError); ok {
		detailed := validationErr.DetailedOutput()
		var lines []string
		collectOutputErrors(detailed, &lines)
		if len(lines) > 0 {
			return strings.Join(lines, "\n  ")
		}
		return validationErr.Error()
	}
	return err.Error()
}

func collectOutputErrors(unit *jsonschema.OutputUnit, lines *[]string) {
	if unit == nil {
		return
	}
	if unit.Error != nil && unit.Error.String() != "" {
		loc := unit.InstanceLocation
		if loc == "" {
			loc = "/"
		}
		*lines = append(*lines, fmt.Sprintf("field %s: %s", loc, unit.Error.String()))
	}
	for i := range unit.Errors {
		collectOutputErrors(&unit.Errors[i], lines)
	}
}
