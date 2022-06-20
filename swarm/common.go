package swarm

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/process"
	"redwood.dev/state"
)

//go:generate mockery --name Transport --output ./mocks/ --case=underscore
type Transport interface {
	process.Interface
	NewPeerConn(ctx context.Context, dialAddr string) (PeerConn, error)
}

//go:generate mockery --name PeerConn --output ./mocks/ --case=underscore
type PeerConn interface {
	PeerEndpoint
	Transport() Transport
	EnsureConnected(ctx context.Context) error
	Close() error
}

var (
	ErrProtocol    = errors.New("protocol error")
	ErrPeerIsSelf  = errors.New("peer is self")
	ErrUnreachable = errors.New("peer unreachable")
	ErrNotReady    = errors.New("not ready")
)

type PeerDialInfo struct {
	TransportName string
	DialAddr      string
}

var _ state.MapKeyScanner = (*PeerDialInfo)(nil)

func (pdi PeerDialInfo) ID() process.PoolUniqueID {
	return pdi.String()
}

func (pdi PeerDialInfo) String() string {
	return strings.Join([]string{pdi.TransportName, pdi.DialAddr}, " ")
}

func (pdi PeerDialInfo) MarshalText() ([]byte, error) {
	return []byte(pdi.TransportName + " " + pdi.DialAddr), nil
}

func (pdi *PeerDialInfo) UnmarshalText(bs []byte) error {
	parts := bytes.SplitN(bs, []byte(" "), 1)
	if len(parts) > 0 {
		pdi.TransportName = string(parts[0])
	}
	if len(parts) > 1 {
		pdi.DialAddr = string(parts[1])
	}
	return nil
}

func (pdi *PeerDialInfo) ScanMapKey(keypath state.Keypath) error {
	parts := bytes.Split(keypath, []byte("|"))
	if len(parts) != 2 {
		return errors.Errorf("bad map keypath for PeerDialInfo: %v", keypath)
	}
	tpt, err := url.QueryUnescape(string(parts[0]))
	if err != nil {
		return err
	}
	dialAddr, err := url.QueryUnescape(string(parts[1]))
	if err != nil {
		return err
	}

	*pdi = PeerDialInfo{tpt, dialAddr}
	return nil
}

func (pdi PeerDialInfo) MapKey() (state.Keypath, error) {
	return state.Keypath(bytes.Join([][]byte{
		[]byte(url.QueryEscape(pdi.TransportName)),
		[]byte(url.QueryEscape(pdi.DialAddr)),
	}, []byte("|"))), nil
}
