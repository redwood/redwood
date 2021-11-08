package testutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
)

func SetupDBTree(t *testing.T) *state.DBTree {
	t.Helper()

	db, err := state.NewDBTree(t.TempDir(), nil)
	require.NoError(t, err)
	return db
}

func SetupDBTreeWithValue(t *testing.T, keypath state.Keypath, val interface{}) *state.DBTree {
	t.Helper()

	db, err := state.NewDBTree(t.TempDir(), nil)
	require.NoError(t, err)

	state := db.State(true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}

func SetupVersionedDBTree(t *testing.T) *state.VersionedDBTree {
	t.Helper()

	db, err := state.NewVersionedDBTree(t.TempDir(), nil)
	require.NoError(t, err)
	return db
}

func SetupVersionedDBTreeWithValue(t *testing.T, keypath state.Keypath, val interface{}) *state.VersionedDBTree {
	t.Helper()

	db, err := state.NewVersionedDBTree(t.TempDir(), nil)
	require.NoError(t, err)

	state := db.StateAtVersion(nil, true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}
