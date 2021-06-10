package mascaras

import (
	"crypto/rand"
	"fmt"
)

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func randstr(n int) (string, error) {

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("get random: %w", err)
	}
	var result string
	for _, c := range b {
		// index が letters の長さに収まるように調整
		result += string(letters[int(c)%len(letters)])
	}
	return result, nil
}
