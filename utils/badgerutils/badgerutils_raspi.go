//go:build raspi

package badgerutils

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/dgraph-io/badger/v3/options"
)

func withPlatformSpecificOpts(opts badger.Options) badger.Options {
	opts.ValueLogLoadingMode = options.FileIO
	return opts
}
