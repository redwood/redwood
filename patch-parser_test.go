package redwood

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/redwood/tree"
)

func TestParsePatch(t *testing.T) {
	patch, err := ParsePatch([]byte(`.text.value[0:0] = "a"`))
	require.NoError(t, err)

	require.Equal(t, tree.Keypath("text/value"), patch.Keypath)
	require.NotNil(t, patch.Range)
	require.Equal(t, int64(0), patch.Range[0])
	require.Equal(t, int64(0), patch.Range[1])
	require.Equal(t, "a", patch.Val)
}
