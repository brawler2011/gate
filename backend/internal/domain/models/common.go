package models

import "errors"

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

type FileEntry struct {
	Path        string `json:"path"`
	IsDirectory bool   `json:"is_directory"`
	Size        int64  `json:"size"`
}
