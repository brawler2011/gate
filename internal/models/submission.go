package models

import (
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type LanguageName int64

const (
	Golang LanguageName = 10
	Cpp    LanguageName = 20
	Python LanguageName = 30
)

func (n LanguageName) Valid() error {
	switch n {
	case Golang, Cpp, Python:
		return nil
	default:
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid language")
	}
}

type State int64

const (
	Saved State = 1 // saved to db

	GotCE State = 101 // compilation error
	GotTL State = 102 // time limit exceeded
	GotML State = 103 // memory limit exceeded
	GotRE State = 104 // runtime error
	GotPE State = 105 // presentation error
	GotWA State = 106 // wrong answer

	Accepted State = 200 // accepted
)

type SubmissionUpdate struct {
	State      State
	Score      int64
	TimeStat   int64
	MemoryStat int64
}

type SubmissionCreation struct {
	Solution  string
	ProblemId uuid.UUID
	ContestId uuid.UUID
	UserId    uuid.UUID
	Language  LanguageName
	Penalty   int64
}

type SolutionsFilter struct {
	Page      int64
	PageSize  int64
	ContestId *uuid.UUID
	UserId    *uuid.UUID
	ProblemId *uuid.UUID
	Language  *LanguageName
	State     *State
	Order     *int64
}

func (f SolutionsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}
