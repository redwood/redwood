package libp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"

	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

type RelayClient struct {
	process.Process
	log.Logger
	store              Store
	host               host.Host
	activeReservations SyncMap[peer.ID, *client.Reservation]
	mbNeedReservation  *utils.Mailbox[peer.AddrInfo]
	chRelays           chan peer.AddrInfo
}

func NewRelayClient(host host.Host, store Store, chRelays chan peer.AddrInfo) *RelayClient {
	return &RelayClient{
		Process:            *process.New("relay client"),
		Logger:             log.NewLogger("libp2p"),
		store:              store,
		host:               host,
		activeReservations: NewSyncMap[peer.ID, *client.Reservation](),
		mbNeedReservation:  utils.NewMailbox[peer.AddrInfo](24),
		chRelays:           chRelays,
	}
}

func (c *RelayClient) Start() error {
	err := c.Process.Start()
	if err != nil {
		return err
	}

	// Add all relays in the store to the mailbox so that we obtain reservations immediately upon startup
	c.mbNeedReservation.DeliverAll(c.store.Relays().Slice())

	// Keep the relay channel used by libp2p's autorelay package full of relays
	c.Process.Go(nil, "fill relay channel", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for _, peerID := range c.activeReservations.Keys() {
				select {
				case <-ctx.Done():
					return
				default:
				}

				reservation, ok := c.activeReservations.Get(peerID)
				if !ok || reservation == nil || reservation.Expiration.Before(time.Now()) {
					c.activeReservations.Delete(peerID)

					addrInfos, _ := MapWithError(c.host.Peerstore().Addrs(peerID), func(multiaddr ma.Multiaddr) (peer.AddrInfo, error) {
						addrInfo, err := peer.AddrInfoFromP2pAddr(multiaddr)
						if err != nil {
							return peer.AddrInfo{}, err
						}
						return *addrInfo, nil
					})
					c.mbNeedReservation.DeliverAll(addrInfos)

				} else {
					select {
					case <-ctx.Done():
						return
					case c.chRelays <- peer.AddrInfo{ID: peerID, Addrs: c.host.Peerstore().Addrs(peerID)}:
					}
				}
			}
			time.Sleep(5 * time.Second)
		}
	})

	c.Process.Go(nil, "acquire reservations", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.mbNeedReservation.Notify():
			}

			for _, addrInfo := range c.mbNeedReservation.RetrieveAll() {
				addrInfo := addrInfo

				c.Process.Go(ctx, "connect to relay "+addrInfo.String(), func(ctx context.Context) {
					// if len(c.host.Network().ConnsToPeer(addrInfo.ID)) == 0 {
					// 	err := c.host.Connect(ctx, addrInfo)
					// 	if err != nil {
					//                        c.Errorw("could not connect to relay", "peer id", addrInfo.ID)
					// 		return
					// 	}
					// 	c.Successw("connected to relay", "peer id", addrInfo.ID)
					// }

					reservation, err := client.Reserve(ctx, c.host, addrInfo)
					if err != nil {
						// c.Errorw("could not acquire relay reservation", "err", err, "relay", addrInfo.ID)
						return
					}
					// c.Successw("acquired relay reservation", "peer id", addrInfo.ID)
					c.activeReservations.Set(addrInfo.ID, reservation)
				})
			}
		}
	})

	return nil
}

func (c *RelayClient) Relays() PeerSet {
	return c.store.Relays()
}

type RelayAndReservation struct {
	AddrInfo    peer.AddrInfo
	Reservation *client.Reservation
}

func (c *RelayClient) RelayReservations() map[peer.ID]RelayAndReservation {
	m := make(map[peer.ID]RelayAndReservation)
	for _, addrInfo := range c.store.Relays().Slice() {
		reservation, _ := c.activeReservations.Get(addrInfo.ID)
		m[addrInfo.ID] = RelayAndReservation{
			AddrInfo:    addrInfo,
			Reservation: reservation,
		}
	}
	return m
}

func (c *RelayClient) AddRelay(addrStr string) error {
	addrInfo, err := addrInfoFromString(addrStr)
	if err != nil {
		return err
	}

	err = c.store.AddRelay(addrStr)
	if err != nil {
		return err
	}

	c.mbNeedReservation.Deliver(addrInfo)
	return nil
}

func (c *RelayClient) RemoveRelay(addrStr string) error {
	return c.store.RemoveRelay(addrStr)
}

func (c *RelayClient) RenewReservation(addrInfo peer.AddrInfo) {
	c.activeReservations.Delete(addrInfo.ID)
	c.mbNeedReservation.Deliver(addrInfo)
}
