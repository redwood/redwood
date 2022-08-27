package generics_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

func TestSet(t *testing.T) {
	set := NewSet[types.Address](nil)

	addr := types.RandomAddress()
	set.Add(addr)

	require.Equal(t, addr, set.Any())
}
