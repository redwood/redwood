//go:build raspi

package badgerutils

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
)

func withPlatformSpecificOpts(opts badger.Options) badger.Options {
	opts.ValueLogLoadingMode = options.FileIO
	return opts
}
