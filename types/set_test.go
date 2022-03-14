package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/types"
)

func TestSet(t *testing.T) {
	set := types.NewSet[types.Address](nil)

	addr := types.RandomAddress()
	set.Add(addr)

	require.Equal(t, addr, set.Any())
}
