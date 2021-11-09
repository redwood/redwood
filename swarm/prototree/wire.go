package prototree

import (
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/swarm/protohush"
	"redwood.dev/tree"
)

type SubscriptionMsg struct {
	StateURI    string          `json:"stateURI"`
	Tx          *tree.Tx        `json:"tx,omitempty"`
	EncryptedTx *EncryptedTx    `json:"encryptedTx,omitempty"`
	State       state.Node      `json:"state,omitempty"`
	Leaves      []state.Version `json:"leaves,omitempty"`
	Error       error           `json:"error,omitempty"`
}

type EncryptedTx = protohush.GroupMessage

type SubscriptionType uint8

const (
	SubscriptionType_Txs SubscriptionType = 1 << iota
	SubscriptionType_States
)

func (t *SubscriptionType) UnmarshalText(bs []byte) error {
	str := strings.Trim(string(bs), `"`)
	parts := strings.Split(str, ",")
	var st SubscriptionType
	for i := range parts {
		switch strings.TrimSpace(parts[i]) {
		case "transactions":
			st |= SubscriptionType_Txs
		case "states":
			st |= SubscriptionType_States
		default:
			return errors.Errorf("bad value for SubscriptionType: %v", str)
		}
	}
	if st == 0 {
		return errors.New("empty value for SubscriptionType")
	}
	*t = st
	return nil
}

func (t SubscriptionType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t SubscriptionType) String() string {
	var strs []string
	if t.Includes(SubscriptionType_Txs) {
		strs = append(strs, "transactions")
	}
	if t.Includes(SubscriptionType_States) {
		strs = append(strs, "states")
	}
	return strings.Join(strs, ",")
}

func (t SubscriptionType) Includes(x SubscriptionType) bool {
	return t&x == x
}
