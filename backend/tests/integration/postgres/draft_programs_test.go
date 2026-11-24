//go:build integration

package postgres_test

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

var testlib = model.DraftJury{Protocol: sandbox.ProtocolTestlib}

func program(language, source string) *model.DraftProgram {
	return &model.DraftProgram{Language: language, Source: source}
}

func checker(language, source string) *model.DraftChecker {
	return &model.DraftChecker{DraftProgram: *program(language, source), DraftJury: testlib}
}

func named(name, language, source string) model.DraftNamedProgram {
	return model.DraftNamedProgram{Name: name, DraftProgram: model.DraftProgram{Language: language, Source: source}}
}

// solutionProgram stays out of the build unless verdicts are given.
func solutionProgram(name, language, source string, verdicts ...string) model.DraftSolution {
	if verdicts == nil {
		verdicts = []string{}
	}
	return model.DraftSolution{DraftNamedProgram: named(name, language, source), Verdicts: verdicts}
}
