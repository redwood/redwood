package braidhttp

import (
	"redwood.dev/types"
)

type StoreBlobResponse struct {
	SHA1 types.Hash `json:"sha1"`
	SHA3 types.Hash `json:"sha3"`
}
