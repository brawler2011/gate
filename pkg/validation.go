package pkg

import (
	"cmp"
	"net/mail"
	"unicode/utf8"
)

func IsLengthBetween(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return IsBetween(length, min, max)
}

func IsBetween[T cmp.Ordered](value, min, max T) bool {
	return value >= min && value <= max
}

func IsEmail(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}
