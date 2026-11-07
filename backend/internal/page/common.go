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

// Request identifies a one-based page. Normalize before calculating the offset.
type Request struct {
	Page     int32
	PageSize int32
}

func (r Request) Normalize() (Request, error) {
	p, size, err := Normalize(r.Page, r.PageSize, 50)
	return Request{Page: p, PageSize: size}, err
}

func (r Request) Offset() int64 { return (int64(r.Page) - 1) * int64(r.PageSize) }

type List[T any] struct {
	Items      []T
	Pagination Pagination
}

type Pagination struct {
	Page       int32 `json:"page"`
	PageSize   int32 `json:"page_size"`
	TotalItems int32 `json:"total_items"`
	TotalPages int32 `json:"total_pages"`
}

func NewPagination(page int32, pageSize int32, total int32) Pagination {
	return Pagination{Page: page, PageSize: pageSize, TotalItems: total, TotalPages: Total(total, pageSize)}
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
