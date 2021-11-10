package blob_test

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"

	"redwood.dev/blob"
	"redwood.dev/types"
	"redwood.dev/utils/badgerutils"
)

type M = map[string]interface{}

func TestReader(t *testing.T) {
	foo := []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Duis magna odio, malesuada sed tortor ut, mollis hendrerit enim. Etiam et nulla lorem. Fusce sollicitudin tortor neque, sed iaculis diam facilisis in. Vivamus pellentesque dapibus magna quis ultricies. Praesent at erat dignissim, pellentesque mauris euismod, condimentum mi. Praesent imperdiet felis dui, a feugiat mi efficitur ut. Vestibulum at semper turpis. Proin in felis sit amet ex rhoncus dapibus sit amet non odio. Sed purus magna, placerat et tortor sed, hendrerit fermentum mi. Etiam auctor vitae lorem ut egestas. Nulla quis justo tristique, facilisis justo sed, malesuada leo. Mauris accumsan bibendum lorem, vitae imperdiet odio semper sit amet. Sed gravida, tellus sit amet scelerisque molestie, leo erat aliquam velit, a finibus lacus leo sit amet tortor.")
	bar := []byte("Integer ac aliquam enim, ut tempus purus. Praesent euismod tempor lorem in pellentesque. Etiam et est sit amet orci mollis scelerisque. Etiam fringilla dictum nulla. Mauris dignissim rhoncus metus ut porttitor. Integer feugiat posuere odio, non tincidunt purus efficitur nec. Duis aliquet volutpat nulla, in ultrices est vehicula ac. Duis fringilla vitae neque a pellentesque. Proin pellentesque dictum tristique.")
	baz := []byte("Vivamus at finibus urna. Aliquam viverra faucibus dolor in pharetra. Sed at varius turpis, eget mattis neque. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Nunc at risus orci. Nam sodales posuere suscipit. Cras in egestas nisl. Nullam ornare orci at neque ullamcorper, non dapibus eros consequat. Quisque eu hendrerit nibh, eget tincidunt quam. Aliquam faucibus tortor eget elit aliquam rutrum. Donec leo nisl, ullamcorper at enim vel, porta ornare elit. Praesent vehicula at elit a gravida.")
	quux := []byte("Donec ultricies sagittis nulla, at posuere justo bibendum ut. Phasellus sit amet tempus nulla. Vivamus eget ex arcu. Maecenas bibendum tortor sed nibh tempus feugiat. Donec ullamcorper mollis arcu non vestibulum. Curabitur porttitor, odio quis lacinia cursus, augue enim vehicula tellus, id consectetur magna dui ut risus. Suspendisse molestie, lacus id ultrices varius, nunc mauris accumsan erat, ornare bibendum nibh nisl eu lectus. Suspendisse nec tellus vitae arcu sollicitudin facilisis congue eu turpis. In tristique erat elit, faucibus pellentesque libero sagittis eget. Aliquam eget nunc erat. Etiam in euismod mi. Nunc vel purus imperdiet, viverra lectus vel, sollicitudin justo.")
	zork := []byte("Phasellus convallis magna in fringilla laoreet. Aliquam ac orci non enim finibus suscipit non eget odio. Morbi finibus ante ut scelerisque maximus. Fusce consectetur id enim ac scelerisque. Nullam vulputate nisi ac est commodo, euismod condimentum ligula rhoncus. Donec eu magna nulla. Pellentesque in finibus est.")

	var badgerOpts badgerutils.OptsBuilder
	store := blob.NewBadgerStore(badgerOpts.ForPath(t.TempDir()))
	err := store.Start()
	require.NoError(t, err)
	defer store.Close()

	content := append(foo, bar...)
	content = append(content, baz...)
	content = append(content, quux...)
	content = append(content, zork...)

	_, _, err = store.StoreBlob(io.NopCloser(bytes.NewReader(content)))
	require.NoError(t, err)

	manifest, err := store.Manifest(blob.ID{HashAlg: types.SHA3, Hash: types.HashBytes(content)})
	require.NoError(t, err)

	err = iotest.TestReader(blob.NewReader(store.HelperGetDB(), manifest), content)
	require.NoError(t, err)
}
