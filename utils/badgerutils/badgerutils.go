package badgerutils

import (
	"time"

	"github.com/dgraph-io/badger/v3"

	"redwood.dev/errors"
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
	// opts.KeepL0InMemory = true                  // @@TODO: make configurable
	// opts.MaxTableSize = 1 << 20
	opts.NumMemtables = 1
	opts.NumLevelZeroTables = 1
	opts.NumLevelZeroTablesStall = 5
	// opts.LevelOneSize = 256 << 10

	// BaseTableSize:       2 << 20,
	// BaseLevelSize:       10 << 20,
	// TableSizeMultiplier: 2,
	// LevelSizeMultiplier: 10,
	// MaxLevels:           7,

	return withPlatformSpecificOpts(opts)
}

func UpdateWithRetryOnConflict(db *badger.DB, fn func(txn *badger.Txn) error) error {
	for {
		err := db.Update(fn)
		if errors.Cause(err) == badger.ErrConflict {
			continue
		} else if err != nil {
			return err
		}
		return nil
	}
}
