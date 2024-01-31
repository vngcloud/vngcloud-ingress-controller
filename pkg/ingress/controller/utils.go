package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

)

func EncodeToValidName(str string) string {
	// Only letters (a-z, A-Z, 0-9, '_', '.', '-') are allowed.
	// the other char will repaced by ":{number}:"
	for _, char := range str {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' || char == '.' || char == '-' {
			continue
		}
		str = strings.ReplaceAll(str, string(char), fmt.Sprintf("-%d-", char))
	}
	return str
}
func DecodeFromValidName(str string) string {
	r, _ := regexp.Compile("-[0-9]+-")
	matchs := r.FindStringSubmatch(str)
	for _, match := range matchs {
		number, _ := strconv.Atoi(match[1 : len(match)-1])
		str = strings.ReplaceAll(str, match, fmt.Sprintf("%c", number))
	}
	return str
}

// hash a string to a string have 10 char
func HashString(str string) string {
	// Create a new SHA-256 hash
	hasher := sha256.New()
	// Write the input string to the hash
	hasher.Write([]byte(str))
	// Sum returns the hash as a byte slice
	hashBytes := hasher.Sum(nil)
	// Truncate the hash to 10 characters
	truncatedHash := hashBytes[:10]
	// Convert the truncated hash to a hex-encoded string
	hashString := hex.EncodeToString(truncatedHash)
	return hashString
}
