package libp2p

import (
	"sort"
	"strings"

	cid "github.com/ipfs/go-cid"
	corepeer "github.com/libp2p/go-libp2p-core/peer"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
	multihash "github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"redwood.dev/swarm"
	"redwood.dev/utils"
)

func cidForString(s string) (cid.Cid, error) {
	pref := cid.NewPrefixV1(cid.Raw, multihash.SHA2_256)
	c, err := pref.Sum([]byte(s))
	if err != nil {
		return cid.Cid{}, errors.Wrap(err, "could not create cid")
	}
	return c, nil
}

func multiaddrsFromPeerInfo(pinfo peerstore.PeerInfo) *utils.SortedStringSet {
	multiaddrs, err := peerstore.InfoToP2pAddrs(&pinfo)
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

	// Sort them
	sort.Slice(multiaddrs, func(i, j int) bool {
		if val := protocolValue(multiaddrs[i], protoIP4); val != "" {
			if val == "127" {
				return true
			} else if val == "192" {
				return true
			}
		} else if protocolValue(multiaddrs[i], protoDNS4) != "" {
			return true
		}
		return false
	})

	// Filter and clean them
	multiaddrStrings := make([]string, 0, len(multiaddrs))
	for _, addr := range multiaddrs {
		if cleaned := cleanLibp2pAddr(addr.String(), pinfo.ID); cleaned != "" {
			multiaddrStrings = append(multiaddrStrings, cleaned)
		}
	}
	return utils.NewSortedStringSet(multiaddrStrings)
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

func cleanLibp2pAddr(addrStr string, peerID peer.ID) string {
	if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
		return ""
	}

	addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

	if !strings.Contains(addrStr, "/p2p/") {
		addrStr = addrStr + "/p2p/" + peerID.Pretty()
	}
	return addrStr
}

func cleanLibp2pAddrs(addrStrs utils.StringSet, peerID peer.ID) utils.StringSet {
	keep := utils.NewStringSet(nil)
	for addrStr := range addrStrs {
		if strings.Index(addrStr, "/ip4/172.") == 0 {
			// continue
			// } else if strings.Index(addrStr, "/ip4/0.0.0.0") == 0 {
			//  continue
			// } else if strings.Index(addrStr, "/ip4/127.0.0.1") == 0 {
			//  continue
		} else if addrStr[:len("/p2p-circuit")] == "/p2p-circuit" {
			continue
		}

		addrStr = strings.Replace(addrStr, "/ipfs/", "/p2p/", 1)

		if !strings.Contains(addrStr, "/p2p/") {
			addrStr = addrStr + "/p2p/" + peerID.Pretty()
		}

		keep.Add(addrStr)
	}
	return keep
}

func protocolValue(addr ma.Multiaddr, proto ma.Protocol) string {
	val, err := addr.ValueForProtocol(proto.Code)
	if err == ma.ErrProtocolNotFound {
		return ""
	}
	return val
}

// func sortLibp2pAddrs(addrs StringSet) SortedStringSet {
//  s := addrs.Slice()
//  sort.Slice(s, func(i, j int) bool {
//         net.ParseIP(s[i])
//         strconv.ParseUint(s[i][])
//         switch s[i][:3] {
//         case "127":
//             return true
//         case "192"
//         }
//  })
//  return utils.NewSortedStringSet(s)
// }

func peerDialInfosFromPeerInfo(pinfo peerstore.PeerInfo) []swarm.PeerDialInfo {
	addrs := multiaddrsFromPeerInfo(pinfo)
	var dialInfos []swarm.PeerDialInfo
	addrs.ForEach(func(addr string) bool {
		dialInfos = append(dialInfos, swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr})
		return true
	})
	return dialInfos
}
