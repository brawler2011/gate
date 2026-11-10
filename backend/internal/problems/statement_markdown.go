package problems

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"regexp"
	"strings"
)

var statementSection = regexp.MustCompile(`(?i)<!--\s*(title|legend|input|output|notes|interaction|scoring)\s*-->`)

func ParseStatementMarkdown(content string) model.Statement {
	var statement model.Statement
	sections := statementSection.FindAllStringSubmatchIndex(content, -1)
	if len(sections) == 0 {
		statement.Legend = strings.TrimSpace(content)
		return statement
	}
	statement.Legend = strings.TrimSpace(content[:sections[0][0]])
	for i, section := range sections {
		end := len(content)
		if i+1 < len(sections) {
			end = sections[i+1][0]
		}
		body := strings.TrimSpace(content[section[1]:end])
		switch strings.ToLower(content[section[2]:section[3]]) {
		case "title":
			statement.Title = body
		case "legend":
			if statement.Legend != "" {
				statement.Legend += "\n\n" + body
			} else {
				statement.Legend = body
			}
		case "input":
			statement.InputFormat = body
		case "output":
			statement.OutputFormat = body
		case "notes":
			statement.Notes = body
		case "interaction":
			statement.Interaction = body
		case "scoring":
			statement.Scoring = body
		}
	}
	return statement
}

func RenderStatementMarkdown(stmt model.Statement) string {
	var sb strings.Builder

	sb.WriteString("<!--legend -->")
	if stmt.Legend != "" {
		sb.WriteString("\n\n")
		sb.WriteString(stmt.Legend)
	}
	if stmt.InputFormat != "" {
		sb.WriteString("\n\n<!-- input -->\n\n")
		sb.WriteString(stmt.InputFormat)
	}
	if stmt.OutputFormat != "" {
		sb.WriteString("\n\n<!-- output -->\n\n")
		sb.WriteString(stmt.OutputFormat)
	}
	if stmt.Interaction != "" {
		sb.WriteString("\n\n<!-- interaction -->\n\n")
		sb.WriteString(stmt.Interaction)
	}
	if stmt.Notes != "" {
		sb.WriteString("\n\n<!-- notes -->\n\n")
		sb.WriteString(stmt.Notes)
	}
	if stmt.Scoring != "" {
		sb.WriteString("\n\n<!-- scoring -->\n\n")
		sb.WriteString(stmt.Scoring)
	}
	sb.WriteString("\n")
	return sb.String()
}
