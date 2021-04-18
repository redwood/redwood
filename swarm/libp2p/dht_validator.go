package libp2p

import (
	"fmt"
)

type blankValidator struct{}

func (blankValidator) Validate(key string, val []byte) error {
	fmt.Println("Validate ~>", key, string(val))
	return nil
}
func (blankValidator) Select(key string, vals [][]byte) (int, error) {
	fmt.Println("SELECT key", key)
	for _, val := range vals {
		fmt.Println("  -", string(val))
	}
	return 0, nil
}
