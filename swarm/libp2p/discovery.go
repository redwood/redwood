package libp2p

// import (
// 	"time"

// 	"golang.org/x/net/context"

// 	corepeer "github.com/libp2p/go-libp2p-core/peer"
// 	"github.com/libp2p/go-libp2p-core/routing"
// 	discovery "github.com/libp2p/go-libp2p-discovery"
// 	p2phost "github.com/libp2p/go-libp2p-host"
// 	peer "github.com/libp2p/go-libp2p-peer"
// 	mdns "github.com/libp2p/go-libp2p/p2p/discovery"

// 	"redwood.dev/process"
// 	"redwood.dev/swarm"
// 	"redwood.dev/utils"
// )

// type PeerDiscovery struct {
// 	process.Process

// 	Libp2pHost p2phost.Host
// 	PeerStore  swarm.PeerStore

// 	RoutingDiscovery    bool
// 	RoutingDiscoveryKey string
// 	ContentRouter       routing.ContentRouting

// 	MDNSDiscovery bool
//     MDNSKey string
// 	mdns          mdns.Service
// }

// func (d Discovery) Start() error {
// 	err := d.Process.Start()
// 	if err != nil {
// 		return err
// 	}

// 	if d.RoutingDiscovery {
// 		// Set up DHT discovery
// 		routingDiscovery := discovery.NewRoutingDiscovery(d.ContentRouter)
// 		discovery.Advertise(d.Process.Ctx(), routingDiscovery, d.RoutingDiscoveryKey)

// 		d.Process.Go("find peers", func(ctx context.Context) {
// 			chPeers, err := routingDiscovery.FindPeers(ctx, d.RoutingDiscoveryKey)
// 			if err != nil {
// 				d.Errorf("error finding peers: %v", err)
// 			}
// 			for peer := range chPeers {
// 				d.onPeerFound("routing", peer)
// 			}
// 		})
// 	}

// 	if d.MDNSDiscovery {
// 		// Set up mDNS discovery
// 		d.mdns, err = mdns.NewMdnsService(d.Process.Ctx(), d.Libp2pHost, 10*time.Second, d.MDNSKey)
// 		if err != nil {
//             d.Close()
// 			return err
// 		}
// 		d.mdns.RegisterNotifee(d)
// 	}

//     return nil
// }

// // HandlePeerFound is the libp2p mDNS peer discovery callback
// func (d Discovery) HandlePeerFound(pinfo corepeer.AddrInfo) {
// 	d.onPeerFound("mDNS", pinfo)
// }

// func (d Discovery) onPeerFound(via string, pinfo corepeer.AddrInfo) {
// 	// Ensure all peers discovered on the libp2p layer are in the peer store
// 	if pinfo.ID == peer.ID("") {
// 		return
// 	}
// 	// dialInfos := peerDialInfosFromPeerInfo(pinfo)
// 	// t.peerStore.AddDialInfos(dialInfos)

//     d.Infof(0, "%v: peer %+v found", via, pinfo.ID.Pretty())

//     t.addPeerInfosToPeerStore([]peerstore.PeerInfo{pinfo})

//     peer := t.makeDisconnectedPeerConn(pinfo)
//     if peer == nil {
//         t.Infof(0, "%v: peer %+v is nil", via, pinfo.ID.Pretty())
//         return
//     }

//     ctx, cancel := utils.CombinedContext(t.Process.Ctx(), 10*time.Second)
//     defer cancel()

//     err := peer.EnsureConnected(ctx)
//     if err != nil {
//         t.Errorf("error connecting to %v peer %v: %v", via, pinfo, err)
//     }
//     // if len(dialInfos) > 0 {
//     //  err := t.libp2pHost.Connect(ctx, pinfo)
//     //  if err != nil {
//     //      t.Errorf("error connecting to peer %v: %v", pinfo.ID.Pretty(), err)
//     //  }
//     // }
// }
