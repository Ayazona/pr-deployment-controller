package internal

import (
	"math/rand"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz"

var seededRand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

// RandomStringWithCharset generates a random string based on a charset
func RandomStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// RandomString generates a random string
func RandomString(length int) string {
	return RandomStringWithCharset(length, charset)
}
