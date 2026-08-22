package pkg

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const passwordAlphabet = "23456789abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"

func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		length = 10
	}
	bytes := make([]byte, length)
	alphabetLen := big.NewInt(int64(len(passwordAlphabet)))
	for i := 0; i < length; i++ {
		idx, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", fmt.Errorf("generate random int: %w", err)
		}
		bytes[i] = passwordAlphabet[idx.Int64()]
	}
	return string(bytes), nil
}
