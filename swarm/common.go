package swarm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/types"
)

//go:generate mockery --name Transport --output ./mocks/ --case=underscore
type Transport interface {
	process.Spawnable
	AwaitReady(ctx context.Context) <-chan bool
	Ready() bool
	NewPeerConn(ctx context.Context, dialAddr string) (PeerConn, error)
}

type BaseTransport struct {
	process.Process
	log.Logger
	chReady chan struct{}
}

func NewBaseTransport(name string) BaseTransport {
	return BaseTransport{
		Process: *process.New(name),
		Logger:  log.NewLogger(name),
		chReady: make(chan struct{}),
	}
}

func (t BaseTransport) MarkReady() {
	close(t.chReady)
}

func (t BaseTransport) Ready() bool {
	select {
	case _, notReady := <-t.chReady:
		return !notReady
	default:
		return false
	}
}

func (t BaseTransport) AwaitReady(ctx context.Context) <-chan bool {
	ch := make(chan bool)

	switch t.Process.State() {
	case process.Closed:
		go func() {
			select {
			case ch <- false:
			case <-ctx.Done():
			}
		}()

	default:
		go func() {
			select {
			case <-t.chReady:
			case <-ctx.Done():
			}

			select {
			case ch <- true:
			case <-ctx.Done():
			}
		}()
	}
	return ch
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
	return pdi.TransportName + " " + pdi.DialAddr
}

func (pdi PeerDialInfo) MarshalText() ([]byte, error) {
	return []byte(pdi.String()), nil
}

func (pdi *PeerDialInfo) UnmarshalText(bs []byte) error {
	parts := bytes.SplitN(bs, []byte(" "), 2)
	if len(parts) > 0 {
		pdi.TransportName = string(parts[0])
	}
	if len(parts) > 1 {
		pdi.DialAddr = string(parts[1])
	}
	return nil
}

func (pdi PeerDialInfo) Bytes() []byte {
	return []byte(pdi.String())
}

func (pdi PeerDialInfo) Copy() PeerDialInfo {
	return pdi
}

func (pdi PeerDialInfo) Marshal() ([]byte, error) {
	return pdi.Bytes(), nil
}

func (pdi *PeerDialInfo) MarshalTo(data []byte) (n int, err error) {
	bs := pdi.Bytes()
	if len(data) < len(bs) {
		return 0, errors.Errorf("buffer too small")
	}
	return copy(data, bs), nil
}

func (pdi *PeerDialInfo) Unmarshal(data []byte) error {
	return pdi.UnmarshalText(data)
}

func (pdi PeerDialInfo) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pdi.TransportName + " " + pdi.DialAddr + `"`), nil
}

func (pdi *PeerDialInfo) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	return pdi.UnmarshalText([]byte(s))
}

func (pdi PeerDialInfo) Equal(other PeerDialInfo) bool {
	return pdi.TransportName == other.TransportName && pdi.DialAddr == other.DialAddr
}

func (pdi PeerDialInfo) Size() int { return len(pdi.String()) }

func (pdi *PeerDialInfo) ScanMapKey(keypath []byte) error {
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

func (pdi PeerDialInfo) MapKey() ([]byte, error) {
	return bytes.Join([][]byte{
		[]byte(url.QueryEscape(pdi.TransportName)),
		[]byte(url.QueryEscape(pdi.DialAddr)),
	}, []byte("|")), nil
}

type gogoprotobufTest interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func NewPopulatedPeerDialInfo(_ gogoprotobufTest) *PeerDialInfo {
	return &PeerDialInfo{TransportName: types.RandomString(32), DialAddr: types.RandomString(32)}
}
