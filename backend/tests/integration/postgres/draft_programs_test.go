//go:build integration

package postgres_test

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func program(language, source string) *model.DraftProgram {
	return &model.DraftProgram{Language: language, Source: source}
}

func named(id uuid.UUID, name, language, source string) model.DraftNamedProgram {
	return model.DraftNamedProgram{ID: id, Name: name, DraftProgram: model.DraftProgram{Language: language, Source: source}}
}
