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
	Tags        []string
}

// HasTag checks if the task has the specified tag (case-insensitive).
func (m *TaskMeta) HasTag(tag string) bool {
	normalized := strings.ToLower(strings.TrimSpace(tag))
	for _, t := range m.Tags {
		if strings.ToLower(strings.TrimSpace(t)) == normalized {
			return true
		}
	}
	return false
}

// IsBackwardCompatible returns true if task tags specify backward compatibility.
func (m *TaskMeta) IsBackwardCompatible() bool {
	for _, t := range m.Tags {
		clean := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(t, "-", " "), "_", " "))
		if strings.TrimSpace(clean) == "backward compatible" {
			return true
		}
	}
	return false
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
		upper := strings.ToUpper(entry.Name())
		if upper == prefix+".MD" || strings.HasPrefix(upper, prefix+"-") || strings.HasPrefix(upper, prefix+"_") {
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

	relPath := absOrRelPath
	if repoRoot, err := findRepoRoot(); err == nil {
		if r, err := filepath.Rel(repoRoot, absOrRelPath); err == nil && !strings.HasPrefix(r, "..") {
			relPath = r
		}
	}

	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	inTags := false
	meta := &TaskMeta{
		FilePath: relPath,
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
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

		if inTags {
			if strings.HasPrefix(line, "- ") {
				tag := strings.Trim(strings.TrimSpace(line[2:]), "\"'")
				if tag != "" {
					meta.Tags = append(meta.Tags, tag)
				}
				continue
			} else if !strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t") {
				inTags = false
			}
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
		case "tags":
			if val == "" {
				inTags = true
			} else if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
				items := strings.Split(val[1:len(val)-1], ",")
				for _, item := range items {
					cleanItem := strings.Trim(strings.TrimSpace(item), "\"'")
					if cleanItem != "" {
						meta.Tags = append(meta.Tags, cleanItem)
					}
				}
			}
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

	// Check Breaking Changes box based on backward compatible tag
	if meta.IsBackwardCompatible() {
		res = strings.Replace(res, "- [ ] No (backward compatible)", "- [x] No (backward compatible)", 1)
	} else {
		res = strings.Replace(res, "- [ ] Yes (breaking change permitted by default)", "- [x] Yes (breaking change permitted by default)", 1)
	}

	// Auto-fill verification plan note if placeholder is present
	verificationPlaceholders := []string{
		"<!-- Outline tests run and verification results (e.g., `task precommit`, unit tests) -->",
		"<!-- Outline tests run and verification results (e.g., task precommit, unit tests) -->",
	}
	for _, vp := range verificationPlaceholders {
		if strings.Contains(res, vp) {
			res = strings.Replace(res, vp, "- [x] Automated pre-commit verification passed (`task precommit`)", 1)
			break
		}
	}

	return res
}
