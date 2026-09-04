package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var taskIDRegex = regexp.MustCompile(`\b(TASK-\d+)\b`)

type TaskMeta struct {
	ID          string
	Title       string
	Status      string
	Type        string
	Description string
	FilePath    string
}

// ExtractTaskID searches for a TASK-XXX pattern in the provided string.
func ExtractTaskID(s string) string {
	match := taskIDRegex.FindStringSubmatch(s)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// FindTaskFile locates the task file in .tasks/ corresponding to the taskID.
func FindTaskFile(repoRoot, taskID string) (string, error) {
	tasksDir := filepath.Join(repoRoot, ".tasks")
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return "", fmt.Errorf("failed to read .tasks directory at %s: %w", tasksDir, err)
	}

	prefix := strings.ToUpper(taskID)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		if strings.EqualFold(entry.Name(), "TEMPLATE.md") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(entry.Name()), prefix) {
			relPath := filepath.Join(".tasks", entry.Name())
			return relPath, nil
		}
	}

	return "", fmt.Errorf("no task file found in %s matching task ID '%s'", tasksDir, taskID)
}

// ParseTaskMeta extracts basic frontmatter metadata from a task markdown file.
func ParseTaskMeta(absOrRelPath string) (*TaskMeta, error) {
	f, err := os.Open(absOrRelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open task file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	meta := &TaskMeta{
		FilePath: absOrRelPath,
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				// End of frontmatter
				break
			}
		}

		if !inFrontmatter {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch strings.ToLower(key) {
		case "id":
			meta.ID = val
		case "title":
			meta.Title = val
		case "status":
			meta.Status = val
		case "type":
			meta.Type = val
		case "description":
			meta.Description = val
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed scanning task file: %w", err)
	}

	if meta.ID == "" {
		return nil, fmt.Errorf("task file %s is missing required 'id' frontmatter", absOrRelPath)
	}

	return meta, nil
}

// RenderPRBody populates the PR template with task metadata and user content.
func RenderPRBody(templateContent string, meta *TaskMeta, customBody string) string {
	res := templateContent

	// Replace task link
	taskPlaceholder := "Resolves / Task: <!-- e.g. .tasks/TASK-002-pr-automation-and-review-flow.md -->"
	replacementTask := fmt.Sprintf("Resolves / Task: [%s](%s) — %s", meta.ID, meta.FilePath, meta.Title)
	if strings.Contains(res, taskPlaceholder) {
		res = strings.Replace(res, taskPlaceholder, replacementTask, 1)
	} else {
		res = strings.Replace(res, "## Task", "## Task\n"+replacementTask, 1)
	}

	// Fill Description
	descPlaceholder := "<!-- Concise summary of changes and non-obvious rationale. Keep focused. -->"
	var descContent strings.Builder
	if meta.Description != "" {
		descContent.WriteString(meta.Description)
	}
	if customBody != "" {
		if descContent.Len() > 0 {
			descContent.WriteString("\n\n")
		}
		descContent.WriteString(customBody)
	}
	if descContent.Len() > 0 && strings.Contains(res, descPlaceholder) {
		res = strings.Replace(res, descPlaceholder, descContent.String(), 1)
	}

	// Check type box if matching
	if meta.Type != "" {
		targetTypeCheck := fmt.Sprintf("- [ ] `%s`:", meta.Type)
		checkedType := fmt.Sprintf("- [x] `%s`:", meta.Type)
		res = strings.Replace(res, targetTypeCheck, checkedType, 1)
	}

	return res
}
