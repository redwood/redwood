package redwood

import (
	"testing"

	rw "github.com/brynbellomy/redwood"
)

func TestValidatorsApplyToNodeUnderWhichTheyExist(t *testing.T) {
	signingKeypair1, err := rw.SigningKeypairFromHex("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19")
	if err != nil {
		panic(err)
	}
	signingKeypair2, err := rw.SigningKeypairFromHex("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef")
	if err != nil {
		panic(err)
	}

	encryptingKeypair1, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}
	encryptingKeypair2, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}

	genesisBytes := `{
        "permissions": {
            "*": {
                "^.*$": {
                    "read":  true,
                    "write": true
                }
            }
        },
        "shrugisland": {
            "talk0": {
                "validator": {"type":"permissions"},
                "permissions": {
                    "96216849c49358b10257cb55b28ea603c874b05e": {
                        "^.*$": {
                            "read": true,
                            "write": true
                        }
                    },
                    "*": {
                        "^.*$": {
                            "read": false,
                            "write": false
                        }
                    }
                }
            }
        }
    }`

	var genesis map[string]interface{}
	err = json.Unmarshal(genesisBytes, &genesis)
	if err != nil {
		panic(err)
	}

	store1, err := rw.NewStore(signingKeypair1.Address(), genesis)
	if err != nil {
		panic(err)
	}
	store2, err := rw.NewStore(signingKeypair2.Address(), genesis)
	if err != nil {
		panic(err)
	}

	c1, err := rw.NewHost(signingKeypair1, encryptingKeypair1, 21231, store1)
	if err != nil {
		panic(err)
	}

	c2, err := rw.NewHost(signingKeypair2, encryptingKeypair2, 21241, store2)
	if err != nil {
		panic(err)
	}

	var (
		txShouldFail = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx1.ID},
			From:    c2.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0 = {"text":"yoooo"}`),
			},
		}

		txShouldPass = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx1.ID},
			From:    c2.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland = {"text":"yoooo"}`),
			},
		}
	)

	err = c2.SignTx(&txShouldFail)
	if err != nil {
		panic(err)
	}

	// @@TODO: expect this to fail
	err = c1.AddTx(txShouldFail)
	if err == nil {
		panic("txShouldFail should have failed!")
	}
}
