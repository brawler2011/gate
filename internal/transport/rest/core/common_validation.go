package handlers

import "unicode/utf8"

func isLengthBetween(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

const (
	minPage         = 1
	minPageSize     = 1
	maxPageSize     = 20
	maxSearchLength = 50
	maxArchiveSize  = 10 * 1024 * 1024 // 10 MB
)
