package tree

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
)

func TestParsePatch(t *testing.T) {
	patch, err := ParsePatch([]byte(`.text.value[0:0] = "a"`))
	require.NoError(t, err)

	require.Equal(t, state.Keypath("text/value"), patch.Keypath)
	require.NotNil(t, patch.Range)
	require.Equal(t, int64(0), patch.Range.Start)
	require.Equal(t, int64(0), patch.Range.End)
	require.Equal(t, "a", patch.Val)
}
