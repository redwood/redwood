package libp2p

import (
	"sort"
	"strings"

	cid "github.com/ipfs/go-cid"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstoreaddr "github.com/libp2p/go-libp2p-peerstore/addr"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"go.uber.org/multierr"

	"redwood.dev/errors"
	"redwood.dev/swarm"
)

func IsValidKey(key []byte) bool {
	_, err := cryptop2p.UnmarshalPrivateKey(key)
	return err == nil
}

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

func multiaddrsFromPeerInfo(pinfo corepeer.AddrInfo) []ma.Multiaddr {
	multiaddrs, err := corepeer.AddrInfoToP2pAddrs(&pinfo)
	if err != nil {
		panic(err)
	}

	// Deduplicate the addrs
	deduped := make(map[string]ma.Multiaddr, len(multiaddrs))
	for _, addr := range multiaddrs {
		deduped[addr.String()] = addr
	}
	multiaddrs = make([]ma.Multiaddr, 0, len(deduped))
	for _, addr := range deduped {
		multiaddrs = append(multiaddrs, addr)
	}
	multiaddrs = filterUselessMultiaddrs(multiaddrs)
	sort.Sort(peerstoreaddr.AddrList(multiaddrs))
	return multiaddrs
}

func filterUselessMultiaddrs(mas []ma.Multiaddr) []ma.Multiaddr {
	multiaddrs := make([]ma.Multiaddr, 0, len(mas))
	for _, addr := range mas {
		if !strings.Contains(addr.String(), "/p2p/") {
			continue
		}
		multiaddrs = append(multiaddrs, addr)
	}
	return multiaddrs
}

func addrInfosFromStrings(ss []string) (infos []corepeer.AddrInfo, err error) {
	for _, s := range ss {
		multiaddr, err := ma.NewMultiaddr(s)
		if err != nil {
			err = multierr.Append(err, errors.Errorf("bad multiaddress (%v): %v", s, err))
			continue
		}
		addrInfo, err := corepeer.AddrInfoFromP2pAddr(multiaddr)
		if err != nil {
			err = multierr.Append(err, errors.Errorf("bad multiaddress (%v): %v", multiaddr, err))
			continue
		}
		infos = append(infos, *addrInfo)
	}
	return infos, err
}

func protocolValue(addr ma.Multiaddr, proto ma.Protocol) string {
	val, err := addr.ValueForProtocol(proto.Code)
	if err == ma.ErrProtocolNotFound {
		return ""
	}
	return val
}

func peerDialInfosFromPeerInfo(pinfo corepeer.AddrInfo) []swarm.PeerDialInfo {
	var dialInfos []swarm.PeerDialInfo
	for _, addr := range multiaddrsFromPeerInfo(pinfo) {
		dialInfos = append(dialInfos, swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr.String()})
	}
	return dialInfos
}

func deviceUniqueID(peerID peer.ID) string {
	return peerID.Pretty()
}
