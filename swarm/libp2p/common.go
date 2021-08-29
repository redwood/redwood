package libp2p

import (
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

var (
	protoDNS4 = ma.ProtocolWithName("dns4")
	protoIP4  = ma.ProtocolWithName("ip4")
	protoIP6  = ma.ProtocolWithName("ip6")

	ErrWrongProtocol = errors.New("wrong protocol")
)
