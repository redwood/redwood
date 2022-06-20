package libp2p

import (
	"context"
	"time"

	corehost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns_legacy"
	ma "github.com/multiformats/go-multiaddr"

	// "github.com/libp2p/go-libp2p/p2p/discovery"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/swarm"
	"redwood.dev/utils"
)

type PeerManager interface {
	process.Interface
	NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error)
	OnPeerFound(via string, pinfo peer.AddrInfo)
	OnConnectedToPeer(network network.Network, conn network.Conn)
	MakeConnectedPeerConn(stream network.Stream) *peerConn
	MakeDisconnectedPeerConn(pinfo peer.AddrInfo) (*peerConn, error)
	EnsureConnectedToAllPeers()
}

type peerManager struct {
	process.Process
	log.Logger

	transport    *transport
	discoveryKey string
	libp2pHost   corehost.Host
	dht          DHT
	store        Store
	peerStore    swarm.PeerStore

	routingDiscovery   *discovery.RoutingDiscovery
	mdns               mdns.Service
	connectToPeersTask *connectToPeersTask
	announcePeersTask  *announcePeersTask
}

func NewPeerManager(transport *transport, discoveryKey string, libp2pHost corehost.Host, dht DHT, store Store, peerStore swarm.PeerStore) *peerManager {
	return &peerManager{
		Process:      *process.New("peer discovery"),
		Logger:       log.NewLogger(TransportName),
		transport:    transport,
		discoveryKey: discoveryKey,
		libp2pHost:   libp2pHost,
		dht:          dht,
		store:        store,
		peerStore:    peerStore,
	}
}

func (pm *peerManager) Start() error {
	err := pm.Process.Start()
	if err != nil {
		return err
	}

	// Set up DHT discovery
	pm.routingDiscovery = discovery.NewRoutingDiscovery(pm.dht)
	discovery.Advertise(pm.Process.Ctx(), pm.routingDiscovery, pm.discoveryKey)

	pm.Process.Go(nil, "find peers", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			func() {
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				chPeers, err := pm.routingDiscovery.FindPeers(ctx, pm.discoveryKey)
				if err != nil {
					pm.Errorf("error finding peers: %v", err)
					return
				}
				for addrInfo := range chPeers {
					pm.OnPeerFound("routing", addrInfo)
				}
				time.Sleep(3 * time.Second)
			}()
		}
	})

	// Set up mDNS discovery
	pm.mdns, err = mdns.NewMdnsService(pm.Process.Ctx(), pm.libp2pHost, 30*time.Second, pm.discoveryKey)
	if err != nil {
		return err
	}
	pm.mdns.RegisterNotifee(pm)

	// Start our periodic tasks
	pm.announcePeersTask = NewAnnouncePeersTask(10*time.Second, pm)
	err = pm.Process.SpawnChild(nil, pm.announcePeersTask)
	if err != nil {
		return err
	}
	pm.announcePeersTask.Enqueue()

	pm.connectToPeersTask = NewConnectToPeersTask(8*time.Second, pm)
	err = pm.Process.SpawnChild(nil, pm.connectToPeersTask)
	if err != nil {
		return err
	}
	pm.connectToPeersTask.Enqueue()

	return nil
}

func (pm *peerManager) Close() error {
	pm.Infof(0, "libp2p peer discovery shutting down")

	err := pm.mdns.Close()
	if err != nil {
		pm.Errorf("error closing libp2p mDNS service: %v", err)
	}
	return pm.Process.Close()
}

func (pm *peerManager) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	addr, err := ma.NewMultiaddr(dialAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse multiaddr '%v'", dialAddr)
	}
	pinfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse PeerInfo from multiaddr '%v'", dialAddr)
	} else if pinfo.ID == pm.libp2pHost.ID() {
		return nil, errors.WithStack(swarm.ErrPeerIsSelf)
	}
	return pm.MakeDisconnectedPeerConn(*pinfo)
}

func (pm *peerManager) MakeConnectedPeerConn(stream network.Stream) *peerConn {
	pinfo := pm.libp2pHost.Peerstore().PeerInfo(stream.Conn().RemotePeer())
	peer := &peerConn{t: pm.transport, pinfo: pinfo, stream: stream}
	duID := deviceUniqueID(pinfo.ID)
	multiaddrs := multiaddrsFromPeerInfo(pinfo)

	peer.PeerEndpoint = pm.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, multiaddrs[0].String()}, duID)
	for _, addr := range multiaddrs[1:] {
		pm.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, addr.String()}, duID)
	}
	return peer
}

func (pm *peerManager) MakeDisconnectedPeerConn(pinfo peer.AddrInfo) (*peerConn, error) {
	peer := &peerConn{t: pm.transport, pinfo: pinfo, stream: nil}
	multiaddrs := multiaddrsFromPeerInfo(pinfo)

	var filtered []ma.Multiaddr
	for _, multiaddr := range multiaddrs {
		// if manet.IsPrivateAddr(multiaddr) {
		//  continue
		// }
		filtered = append(filtered, multiaddr)
	}
	if len(multiaddrs) == 0 {
		return nil, swarm.ErrUnreachable
	}

	duID := deviceUniqueID(pinfo.ID)

	peer.PeerEndpoint = pm.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddrs[0].String()}, duID)
	for _, addr := range multiaddrs[1:] {
		pm.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr.String()}, duID)
	}
	// for _, addr := range multiaddrs {
	//      pm.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: TransportName, DialAddr: addr.String()}, duID)
	// }
	// peerDevice := pm.peerStore.PeerWithDeviceUniqueID(duID)
	return peer, nil
}

// OnConnectedToPeer is called when the libp2p host connects to a remote peer
func (pm *peerManager) OnConnectedToPeer(network network.Network, conn network.Conn) {
	p2pComponent, err := ma.NewMultiaddr("/p2p/" + conn.RemotePeer().Pretty())
	if err != nil {
		return
	}
	addrInfo, err := peer.AddrInfoFromP2pAddr(conn.RemoteMultiaddr().Encapsulate(p2pComponent))
	if err != nil {
		return
	}

	pm.Debugf("libp2p connected: %v", addrInfo.String())
	pm.OnPeerFound("incoming", *addrInfo)
}

// HandlePeerFound is the libp2p mDNS peer discovery callback
func (pm *peerManager) HandlePeerFound(pinfo peer.AddrInfo) {
	pm.OnPeerFound("mDNS", pinfo)
}

func (pm *peerManager) OnPeerFound(via string, pinfo peer.AddrInfo) {
	if pinfo.ID == peer.ID("") { // Bad data, ignore
		return
	} else if pinfo.ID == pm.libp2pHost.ID() { // Self, ignore
		return
	} else if pm.store.StaticRelays().ContainsPeerID(pinfo.ID) {
		return
	}

	// Add to peer store (unless it's a static relay or local/private IP)
	// Once libp2p peers are added to the peer store, we automatically attempt
	// to connect to them (see `connectToPeersTask`)
	for _, multiaddr := range multiaddrsFromPeerInfo(pinfo) {
		// if manet.IsPrivateAddr(multiaddr) {
		//  continue
		// }

		dialInfo := swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddr.String()}
		if !pm.peerStore.IsKnownPeer(dialInfo) {
			pm.Infof(0, "%v: peer (%v) %+v found", via, pinfo.ID, multiaddr)
		}
		pm.peerStore.AddDialInfo(dialInfo, deviceUniqueID(pinfo.ID))

		if !multiaddrIsRelayed(multiaddr) {
			for _, relayAddrInfo := range pm.store.StaticRelays().Slice() {
				multiaddr, err := ma.NewMultiaddr("/p2p/" + relayAddrInfo.ID.String() + "/p2p-circuit/p2p/" + pinfo.ID.String())
				if err != nil {
					continue
				}
				dialInfo := swarm.PeerDialInfo{TransportName: TransportName, DialAddr: multiaddr.String()}
				pm.peerStore.AddDialInfo(dialInfo, deviceUniqueID(pinfo.ID))
			}
		}
	}
}

func (pm *peerManager) EnsureConnectedToAllPeers() {
	pm.connectToPeersTask.Enqueue()
}

type connectToPeersTask struct {
	process.PeriodicTask
	log.Logger
	pm       *peerManager
	interval time.Duration
}

func NewConnectToPeersTask(
	interval time.Duration,
	pm *peerManager,
) *connectToPeersTask {
	t := &connectToPeersTask{
		Logger:   log.NewLogger("libp2p"),
		pm:       pm,
		interval: interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("ConnectToPeers", utils.NewStaticTicker(interval), t.connectToPeers)
	return t
}

func (t *connectToPeersTask) connectToPeers(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "connect to peers")
	defer child.AutocloseWithCleanup(cancel)

	for _, addrInfo := range t.pm.store.StaticRelays().Slice() {
		if len(t.pm.libp2pHost.Network().ConnsToPeer(addrInfo.ID)) > 0 {
			continue
		}

		addrInfo := addrInfo
		child.Go(ctx, "connect to static relay "+addrInfo.String(), func(ctx context.Context) {
			err := t.pm.libp2pHost.Connect(ctx, addrInfo)
			if err != nil {
				return
			}
			t.pm.Successf("connected to static relay %v", addrInfo)
		})
	}

	for _, endpoint := range t.pm.peerStore.PeersFromTransport(TransportName) {
		peerID, err := peer.Decode(endpoint.DeviceUniqueID())
		if err != nil {
			continue
		} else if peerID == t.pm.libp2pHost.ID() {
			continue
		}

		if len(t.pm.libp2pHost.Network().ConnsToPeer(peerID)) > 0 {
			continue
		}

		addrInfo := t.pm.libp2pHost.Peerstore().PeerInfo(peerID)
		if len(addrInfo.Addrs) == 0 {
			continue
		}

		peerConn, err := t.pm.MakeDisconnectedPeerConn(addrInfo)
		if err != nil {
			t.Errorf("while making disconnected peer conn (%v): %v", addrInfo.ID.Pretty(), err)
			continue
		}

		if !peerConn.Ready() || !peerConn.Dialable() {
			continue
		}

		child.Go(ctx, "ensure connected to "+peerID.Pretty(), func(ctx context.Context) {
			err := peerConn.EnsureConnected(ctx)
			if err != nil {
				return
			}
			t.pm.Successf("connected to libp2p peer %v", addrInfo)
		})

	}
}

type announcePeersTask struct {
	process.PeriodicTask
	log.Logger
	pm       *peerManager
	interval time.Duration
}

func NewAnnouncePeersTask(
	interval time.Duration,
	pm *peerManager,
) *announcePeersTask {
	t := &announcePeersTask{
		Logger:   log.NewLogger(TransportName),
		pm:       pm,
		interval: interval,
	}
	t.PeriodicTask = *process.NewPeriodicTask("AnnouncePeersTask", utils.NewStaticTicker(interval), t.announcePeers)
	return t
}

func (t *announcePeersTask) announcePeers(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, t.interval)

	child := t.Process.NewChild(ctx, "announce peers")
	defer child.AutocloseWithCleanup(cancel)

	var allDialInfos []swarm.PeerDialInfo
	for dialInfo := range t.pm.peerStore.AllDialInfos() {
		if dialInfo.TransportName == TransportName {
			addrInfo, err := addrInfoFromString(dialInfo.DialAddr)
			if err != nil {
				continue
			}
			// Cull static relays from the peer store
			if t.pm.store.StaticRelays().ContainsPeerID(addrInfo.ID) {
				_ = t.pm.peerStore.RemovePeers([]string{addrInfo.ID.Pretty()})
				continue
			}
		} else {
			allDialInfos = append(allDialInfos, dialInfo)
		}
	}

	for _, peerDevice := range t.pm.peerStore.PeersFromTransport(TransportName) {
		if !peerDevice.Ready() || !peerDevice.Dialable() {
			continue
		} else if peerDevice.DialInfo().TransportName != TransportName {
			continue
		}

		peerDevice := peerDevice

		child.Go(ctx, "announce peers", func(ctx context.Context) {
			swarmPeerConn, err := t.pm.NewPeerConn(ctx, peerDevice.DialInfo().DialAddr)
			if errors.Cause(err) == swarm.ErrPeerIsSelf {
				return
			} else if err != nil {
				t.Warnf("error creating new %v peerConn: %v", TransportName, err)
				return
			}
			defer swarmPeerConn.Close()

			libp2pPeerConn := swarmPeerConn.(*peerConn)

			err = libp2pPeerConn.EnsureConnected(ctx)
			if errors.Cause(err) == errors.ErrConnection {
				return
			} else if err != nil {
				t.Warnf("error connecting to %v peer (%v): %v", TransportName, peerDevice.DialInfo().DialAddr, err)
				return
			}

			err = libp2pPeerConn.AnnouncePeers(ctx, allDialInfos)
			if err != nil {
				// t.Errorf("error writing to peerConn: %+v", err)
			}
		})
	}
}
