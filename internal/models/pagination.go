package models

type Pagination struct {
	Page  int64 `json:"page"`
	Total int64 `json:"total"`
}

func Total(count int64, pageSize int64) int64 {
	if count%pageSize == 0 {
		return count / pageSize
	}
	return count/pageSize + 1
}
