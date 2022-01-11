package badgerutils

import (
	"time"

	"github.com/dgraph-io/badger/v2"

	"redwood.dev/utils"
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
	opts.IndexCacheSize = int64(100 * utils.MB) // @@TODO: make configurable
	opts.KeepL0InMemory = true                  // @@TODO: make configurable
	opts.MaxTableSize = 1 << 20
	opts.NumMemtables = 1
	opts.NumLevelZeroTables = 1
	opts.NumLevelZeroTablesStall = 5
	opts.LevelOneSize = 256 << 10

	return withPlatformSpecificOpts(opts)
}
