package testutils

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/state"
)

func SetupDBTree(t *testing.T) *state.DBTree {
	t.Helper()

	i := rand.Int()
	db, err := state.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)
	return db
}

func SetupDBTreeWithValue(t *testing.T, keypath state.Keypath, val interface{}) *state.DBTree {
	t.Helper()

	i := rand.Int()

	db, err := state.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)

	state := db.State(true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}

func SetupVersionedDBTree(t *testing.T) *state.VersionedDBTree {
	t.Helper()

	i := rand.Int()
	db, err := state.NewVersionedDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)
	return db
}

func SetupVersionedDBTreeWithValue(t *testing.T, keypath state.Keypath, val interface{}) *state.VersionedDBTree {
	t.Helper()

	i := rand.Int()

	db, err := state.NewVersionedDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)

	state := db.StateAtVersion(nil, true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}
