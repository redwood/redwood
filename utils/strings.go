package utils

import (
	"math/rand"
	"strconv"
)

func FilterEmptyStrings(s []string) []string {
	var filtered []string
	for i := range s {
		if s[i] == "" {
			continue
		}
		filtered = append(filtered, s[i])
	}
	return filtered
}

func RandomNumberString() string {
	return strconv.Itoa(rand.Intn(8999) + 1000)
}

func TrimStringToLen(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}
