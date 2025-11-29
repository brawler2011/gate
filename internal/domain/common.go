package domain

type Pagination struct {
	Page  int64 `json:"page"`
	Total int64 `json:"total"`
}

func NewPagination(page int64, pageSize int64, total int64) Pagination {
	return Pagination{
		Page:  page,
		Total: Total(total, pageSize),
	}
}

func Total(count int64, pageSize int64) int64 {
	if pageSize == 0 {
		return 0
	}
	if count%pageSize == 0 {
		return count / pageSize
	}
	return count/pageSize + 1
}

