package badgerutils

import (
	"time"

	"github.com/dgraph-io/badger/v2"
)

type OptsBuilder struct {
	encryptionKey                 []byte
	encryptionKeyRotationInterval time.Duration
}

func (b OptsBuilder) WithEncryption(encryptionKey []byte, encryptionKeyRotationInterval time.Duration) OptsBuilder {
	b.encryptionKey = encryptionKey
	b.encryptionKeyRotationInterval = encryptionKeyRotationInterval
	return b
}

func (b OptsBuilder) ForPath(dbFilename string) badger.Options {
	opts := badger.DefaultOptions(dbFilename)
	opts.Logger = nil
	opts.EncryptionKey = b.encryptionKey
	opts.EncryptionKeyRotationDuration = b.encryptionKeyRotationInterval
	opts.IndexCacheSize = 100 << 20 // @@TODO: make configurable
	opts.KeepL0InMemory = true      // @@TODO: make configurable

	return withPlatformSpecificOpts(opts)
}
