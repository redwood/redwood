package tree

import (
	"net/url"

	"redwood.dev/state"
	"redwood.dev/tree/pb"
	"redwood.dev/types"
)

var (
	GenesisTxID = state.VersionFromString("genesis")
	EmptyHash   = types.Hash{}
)

type Tx = pb.Tx
type Patch = pb.Patch
type TxStatus = pb.TxStatus

var (
	TxStatusUnknown   = pb.TxStatusUnknown
	TxStatusInMempool = pb.TxStatusInMempool
	TxStatusInvalid   = pb.TxStatusInvalid
	TxStatusValid     = pb.TxStatusValid

	ParsePatch     = pb.ParsePatch
	ParsePatchPath = pb.ParsePatchPath
)

type StateURI string

func (s StateURI) MapKey() (state.Keypath, error) {
	return state.Keypath(url.QueryEscape(string(s))), nil
}

func (s *StateURI) ScanMapKey(keypath state.Keypath) error {
	stateURI, err := url.QueryUnescape(string(keypath))
	if err != nil {
		return err
	}
	*s = StateURI(stateURI)
	return nil
}
