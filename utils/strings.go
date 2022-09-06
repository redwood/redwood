package utils

import (
	"math/rand"
	"strconv"
)

func RandomNumberString() string {
	return strconv.Itoa(rand.Intn(8999) + 1000)
}
