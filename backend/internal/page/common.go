package page

import (
	"errors"

	"github.com/brawler2011/gate/backend/pkg"
)

// Validate checks pagination bounds without applying defaults.
func Validate(page, size int32) error {
	if page < 1 || size < 1 || size > 100 {
		return pkg.ErrBadInput
	}
	return nil
}

// Normalize applies defaults to zero values and validates the resulting pagination.
func Normalize(page, size, defaultSize int32) (int32, int32, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = defaultSize
	}
	return page, size, Validate(page, size)
}

type Pagination struct {
	Page  int32 `json:"page"`
	Total int32 `json:"total"`
}

func NewPagination(page int32, pageSize int32, total int32) Pagination {
	return Pagination{
		Page:  page,
		Total: Total(total, pageSize),
	}
}

func Total(count int32, pageSize int32) int32 {
	if count%pageSize == 0 {
		return count / pageSize
	}
	return count/pageSize + 1
}

type SortOrder = string

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

func SortOrderValidate(o SortOrder) error {
	if o == SortOrderAsc || o == SortOrderDesc {
		return nil
	}

	return errors.New("sort order must be one of 'asc' or 'desc'")
}
