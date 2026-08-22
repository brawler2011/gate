package templates

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed testdata/*
var templatesFS embed.FS

type BuiltinTemplate struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ProblemType string `json:"problem_type"`
	DirName     string `json:"-"`
}

var builtinTemplates = []BuiltinTemplate{
	{
		ID:          "builtin:a-plus-b",
		Title:       "Стандартная задача (A+B)",
		Description: "Классическая задача со стандартным вводом-выводом, чекером на testlib.h и валидатором тестов",
		ProblemType: "pass-fail",
		DirName:     "a-plus-b",
	},
	{
		ID:          "builtin:interactive-guess",
		Title:       "Интерактивная задача (Угадай число)",
		Description: "Интерактивная задача с интерактором и чекером на testlib.h",
		ProblemType: "interactive",
		DirName:     "interactive-guess",
	},
	{
		ID:          "builtin:subtasks-groups",
		Title:       "Задача с подзадачами (Баллы за группы)",
		Description: "Задача с разделением тестов на группы (подзадачи) и начислением баллов",
		ProblemType: "scoring",
		DirName:     "subtasks-groups",
	},
}

func ListBuiltinTemplates() []BuiltinTemplate {
	res := make([]BuiltinTemplate, len(builtinTemplates))
	copy(res, builtinTemplates)
	return res
}

func GetBuiltinTemplate(id string) (*BuiltinTemplate, error) {
	for _, t := range builtinTemplates {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unknown builtin template: %s", id)
}

func GetBuiltinTemplateZip(id string) ([]byte, error) {
	tmpl, err := GetBuiltinTemplate(id)
	if err != nil {
		return nil, err
	}

	subFS, err := fs.Sub(templatesFS, "testdata/"+tmpl.DirName)
	if err != nil {
		return nil, fmt.Errorf("failed to open embedded template directory: %w", err)
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	err = fs.WalkDir(subFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(subFS, path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}

		// Normalize zip path separators to forward slash
		cleanPath := strings.ReplaceAll(path, "\\", "/")
		w, err := zipWriter.Create(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %w", cleanPath, err)
		}

		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("failed to write zip entry %s: %w", cleanPath, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory: %w", err)
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize zip writer: %w", err)
	}

	return buf.Bytes(), nil
}
