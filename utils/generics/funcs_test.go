package generics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMin(t *testing.T) {
	min, ok := Min(1, 19, 4, 43)
	require.Equal(t, true, ok)
	require.Equal(t, 1, min)
}
