package testutils

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/tree"
)

func SetupDBTree(t *testing.T) *tree.DBTree {
	t.Helper()

	i := rand.Int()
	db, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)
	return db
}

func SetupDBTreeWithValue(t *testing.T, keypath tree.Keypath, val interface{}) *tree.DBTree {
	t.Helper()

	i := rand.Int()

	db, err := tree.NewDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)

	state := db.State(true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}

func SetupVersionedDBTree(t *testing.T) *tree.VersionedDBTree {
	t.Helper()

	i := rand.Int()
	db, err := tree.NewVersionedDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)
	return db
}

func SetupVersionedDBTreeWithValue(t *testing.T, keypath tree.Keypath, val interface{}) *tree.VersionedDBTree {
	t.Helper()

	i := rand.Int()

	db, err := tree.NewVersionedDBTree(fmt.Sprintf("/tmp/tree-badger-test-%v", i))
	require.NoError(t, err)

	state := db.StateAtVersion(nil, true)
	defer state.Save()

	err = state.Set(keypath, nil, val)
	require.NoError(t, err)

	return db
}
