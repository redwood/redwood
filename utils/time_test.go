package utils_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/utils"
)

func TestExponentialBackoff(t *testing.T) {
	t.Run("it should be .Ready() at initialization", func(t *testing.T) {
		eb := utils.ExponentialBackoff{Min: 10 * time.Second, Max: 1 * time.Minute}
		ready, _ := eb.Ready()
		require.True(t, ready)
	})

	t.Run("it should not be .Ready() immediately after .Next() is called", func(t *testing.T) {
		eb := utils.ExponentialBackoff{Min: 10 * time.Second, Max: 1 * time.Minute}
		eb.Next()
		ready, _ := eb.Ready()
		require.False(t, ready)
	})
}
