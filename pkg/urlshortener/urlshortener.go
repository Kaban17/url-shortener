package urlshortener

import (
	"crypto/sha256"
	"fmt"
	"strconv"
)

func Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))[:10]
}
func HexToDecimal(hex string) (uint64, error) {
	return strconv.ParseUint(hex, 16, 64)
}
func IDtoShortURL(id uint64) string {
	abc := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var shortURL string
	for id > 0 {
		newID, remainder := id/62, id%62
		shortURL = string(abc[remainder]) + shortURL
		id = newID
	}
	return shortURL
}
func URLtoShortURL(url string) (string, error) {
	id, err := HexToDecimal(Hash(url))

	return IDtoShortURL(id), err
}
