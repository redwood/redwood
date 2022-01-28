package libp2p

import (
	"sort"
	"strings"
	"sync"

	cid "github.com/ipfs/go-cid"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	peerstoreaddr "github.com/libp2p/go-libp2p-peerstore/addr"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"go.uber.org/multierr"

	"redwood.dev/errors"
	"redwood.dev/swarm"
)

var (
	protoDNS4 = ma.ProtocolWithName("dns4")
	protoIP4  = ma.ProtocolWithName("ip4")
	protoIP6  = ma.ProtocolWithName("ip6")

	ErrWrongProtocol = errors.New("wrong protocol")
)

type PeerSet struct {
	set map[corepeer.ID]corepeer.AddrInfo
	mu  sync.RWMutex
}

func NewPeerSet() PeerSet {
	return PeerSet{set: make(map[corepeer.ID]corepeer.AddrInfo)}
}

func NewPeerSetFromStrings(ss []string) (PeerSet, error) {
	addrInfos, err := addrInfosFromStrings(ss)
	if err != nil {
		return PeerSet{}, err
	}
	set := NewPeerSet()
	for _, addrInfo := range addrInfos {
		if existing, exists := set.set[addrInfo.ID]; exists {
			existing.Addrs = append(existing.Addrs, addrInfo.Addrs...)
			set.set[addrInfo.ID] = existing
		} else {
			set.set[addrInfo.ID] = addrInfo
		}
	}
	return set, nil
}

// func NewPeerSetFromPeerDialInfos(dialInfos []swarm.PeerDialInfo) (PeerSet, error) {
// 	dialAddrs := make([]string, len(dialInfos))
// 	for i, dialInfo := range dialInfos {
// 		dialAddrs[i] = dialInfo.DialAddr
// 	}
// 	return NewPeerSetFromStrings(dialAddrs)
// }

func (set *PeerSet) Peers() PeerSet {
	return set.Copy()
}

func (set *PeerSet) AddString(s string) error {
	set.mu.Lock()
	defer set.mu.Unlock()

	addrInfo, err := addrInfoFromString(s)
	if err != nil {
		return err
	}
	set.set[addrInfo.ID] = addrInfo
	return nil
}

func (set *PeerSet) RemoveString(s string) error {
	set.mu.Lock()
	defer set.mu.Unlock()

	addrInfo, err := addrInfoFromString(s)
	if err != nil {
		return err
	}
	delete(set.set, addrInfo.ID)
	return nil
}

func (set PeerSet) ContainsPeerID(peerID corepeer.ID) bool {
	set.mu.RLock()
	defer set.mu.RUnlock()

	_, exists := set.set[peerID]
	return exists
}

func (set PeerSet) ContainsPeerIDString(peerIDStr string) bool {
	peerID, err := corepeer.Decode(peerIDStr)
	if err != nil {
		return false
	}
	return set.ContainsPeerID(peerID)
}

func (set PeerSet) Slice() []corepeer.AddrInfo {
	set.mu.RLock()
	defer set.mu.RUnlock()

	slice := make([]corepeer.AddrInfo, len(set.set))
	i := 0
	for _, v := range set.set {
		slice[i] = v
		i++
	}
	return slice
}

func (set PeerSet) MultiaddrStrings() []string {
	var strs []string
	for _, addrInfo := range set.Slice() {
		for _, addr := range addrInfo.Addrs {
			strs = append(strs, addr.String())
		}
	}
	return strs
}

func (set PeerSet) Copy() PeerSet {
	set.mu.RLock()
	defer set.mu.RUnlock()

	newSet := make(map[corepeer.ID]corepeer.AddrInfo, len(set.set))
	for k, v := range set.set {
		newSet[k] = v
	}
	return PeerSet{set: newSet}
}

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

func addrInfoFromString(s string) (corepeer.AddrInfo, error) {
	multiaddr, err := ma.NewMultiaddr(s)
	if err != nil {
		return corepeer.AddrInfo{}, errors.Wrapf(err, "bad multiaddress (%v)", s)
	}
	addrInfo, err := corepeer.AddrInfoFromP2pAddr(multiaddr)
	if err != nil {
		return corepeer.AddrInfo{}, errors.Wrapf(err, "bad multiaddress (%v)", s)
	}
	return *addrInfo, nil
}

func addrInfosFromStrings(ss []string) (infos []corepeer.AddrInfo, err error) {
	for _, s := range ss {
		addrInfo, err := addrInfoFromString(s)
		if err != nil {
			err = multierr.Append(err, errors.Errorf("bad multiaddress (%v): %v", s, err))
			continue
		}
		infos = append(infos, addrInfo)
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

func peerDialInfosFromAddrInfo(pinfo corepeer.AddrInfo) []swarm.PeerDialInfo {
	var dialInfos []swarm.PeerDialInfo
	for _, addr := range multiaddrsFromPeerInfo(pinfo) {
		dialInfos = append(dialInfos, swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr.String()})
	}
	return dialInfos
}

func peerIDFromMultiaddr(multiaddr ma.Multiaddr) (peerID corepeer.ID, ok bool) {
	ma.ForEach(multiaddr, func(cmpt ma.Component) bool {
		if cmpt.Protocol().Code == ma.P_P2P {
			id, err := corepeer.Decode(cmpt.Value())
			if err != nil {
				return true
			}
			peerID = id
			ok = true
		}
		return true
	})
	return
}

func splitRelayAndPeer(multiaddr ma.Multiaddr) (relay, peer ma.Multiaddr) {
	relay, peer = ma.SplitFunc(multiaddr, func(c ma.Component) bool {
		return c.Protocol().Code == ma.P_CIRCUIT
	})
	if peer == nil {
		return nil, relay
	} else {
		_, peer = ma.SplitFirst(peer)
		return relay, peer
	}
}

func multiaddrIsRelayed(multiaddr ma.Multiaddr) (is bool) {
	ma.ForEach(multiaddr, func(cmpt ma.Component) bool {
		if cmpt.Protocol().Code == ma.P_CIRCUIT {
			is = true
			return false
		}
		return true
	})
	return
}

func deviceUniqueID(peerID corepeer.ID) string {
	return peerID.Pretty()
}
