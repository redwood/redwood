package vault_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/vault"
)

func TestStore(t *testing.T) {
	root := t.TempDir()
	store := vault.NewDiskStore(root)
	defer os.RemoveAll(root)

	data := []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis magna odio, malesuada sed tortor ut, mollis hendrerit enim. Etiam et nulla lorem. Fusce sollicitudin tortor neque, sed iaculis diam facilisis in. Vivamus pellentesque dapibus magna quis ultricies. Praesent at erat dignissim, pellentesque mauris euismod, condimentum mi. Praesent imperdiet felis dui, a feugiat mi efficitur ut. Vestibulum at semper turpis. Proin in felis sit amet ex rhoncus dapibus sit amet non odio. Sed purus magna, placerat et tortor sed, hendrerit fermentum mi. Etiam auctor vitae lorem ut egestas. Nulla quis justo tristique, facilisis justo sed, malesuada leo. Mauris accumsan bibendum lorem, vitae imperdiet odio semper sit amet. Sed gravida, tellus sit amet scelerisque molestie, leo erat aliquam velit, a finibus lacus leo sit amet tortor.")

	collectionID := "foo"
	itemID := "bar"

	err := store.Upsert(collectionID, itemID, bytes.NewReader(data))
	require.NoError(t, err)

	collections, err := store.Collections()
	require.NoError(t, err)
	require.Len(t, collections, 1)
	require.Equal(t, collectionID, collections[0])

	items, err := store.Items(collectionID, 0, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, itemID, items[0])

	bs, err := ioutil.ReadFile(filepath.Join(root, collectionID, "manifest.json"))
	require.NoError(t, err)
	fmt.Println(string(bs))
}
