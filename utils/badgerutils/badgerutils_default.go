//go:build !raspi

package badgerutils

import (
	"github.com/dgraph-io/badger/v2"
)

func withPlatformSpecificOpts(opts badger.Options) badger.Options {
	return opts
}
