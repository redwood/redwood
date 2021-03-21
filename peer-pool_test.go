package redwood_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"redwood.dev"
	"redwood.dev/mocks"
	"redwood.dev/testutils"
	"redwood.dev/types"
	"redwood.dev/utils"
)

func TestPeerPool(t *testing.T) {
	t.Parallel()

	const (
		concurrentConns = 3
	)

	var (
		peer1   = new(mocks.Peer)
		peer2   = new(mocks.Peer)
		peer3   = new(mocks.Peer)
		peer4   = new(mocks.Peer)
		peers   = []redwood.Peer{peer1, peer2, peer3, peer4}
		chPeers = make(chan redwood.Peer)
	)

	peer1.On("DialInfo").Return(redwood.PeerDialInfo{TransportName: "test", DialAddr: "1"})
	peer2.On("DialInfo").Return(redwood.PeerDialInfo{TransportName: "test", DialAddr: "2"})
	peer3.On("DialInfo").Return(redwood.PeerDialInfo{TransportName: "test", DialAddr: "3"})
	peer4.On("DialInfo").Return(redwood.PeerDialInfo{TransportName: "test", DialAddr: "4"})
	peer1.On("Ready").Return(true)
	peer2.On("Ready").Return(true)
	peer3.On("Ready").Return(true)
	peer4.On("Ready").Return(true)
	peer1.On("Addresses").Return([]types.Address{{0x1}})
	peer2.On("Addresses").Return([]types.Address{{0x2}})
	peer3.On("Addresses").Return([]types.Address{{0x3}})
	peer4.On("Addresses").Return([]types.Address{{0x4}})
	peer1.On("Close").Return(nil)
	peer2.On("Close").Return(nil)
	peer3.On("Close").Return(nil)
	peer4.On("Close").Return(nil)

	var numProvidersRequests uint32
	pool := redwood.NewPeerPool(concurrentConns, func(ctx context.Context) (<-chan redwood.Peer, error) {
		atomic.AddUint32(&numProvidersRequests, 1)
		return chPeers, nil
	})

	activePeers := make(map[redwood.Peer]struct{})

	t.Run("requests a providers channel on startup", func(t *testing.T) {
		require.Eventually(t, func() bool { return atomic.LoadUint32(&numProvidersRequests) == 1 }, 1*time.Second, 10*time.Millisecond)
	})

	t.Run("does not return peers before the channel begins to send", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		peer, err := pool.GetPeer(ctx)
		require.Error(t, err)
		require.Nil(t, peer)
	})

	t.Run("only returns as many peers as are available", func(t *testing.T) {
		go func() {
			select {
			case chPeers <- peers[0]:
			case <-time.After(3 * time.Second):
				t.Fatal("could not send")
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Equal(t, peers[0], peer)
		activePeers[peer] = struct{}{}

		peer, err = pool.GetPeer(ctx)
		require.Error(t, err)
		require.Nil(t, peer)
	})

	t.Run("returns up to `concurrentConns` peers", func(t *testing.T) {
		go func() {
			for i := 1; i < 3; i++ {
				select {
				case chPeers <- peers[i]:
				case <-time.After(3 * time.Second):
					t.Fatal("could not send")
				}
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Contains(t, peers[1:3], peer)
		activePeers[peer] = struct{}{}

		peer, err = pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Contains(t, peers[1:3], peer)
		activePeers[peer] = struct{}{}
	})

	t.Run("blocks after `concurrentConns` peers have been obtained", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.Error(t, err)
		require.Nil(t, peer)
	})

	t.Run("when a peer is returned with no strike, allows it to be given out again", func(t *testing.T) {
		pool.ReturnPeer(peer1, false)
		delete(activePeers, peer1)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Contains(t, peers, peer)
		activePeers[peer] = struct{}{}

		peer, err = pool.GetPeer(ctx)
		require.Error(t, err)
		require.Nil(t, peer)
	})

	t.Run("when a peer is returned with a strike, it will not be given out again", func(t *testing.T) {
		pool.ReturnPeer(peer3, true)
		delete(activePeers, peer3)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.Error(t, err)
		require.Nil(t, peer)
	})

	t.Run("after all peers without strikes have been returned, they can all be given out again", func(t *testing.T) {
		for peer := range activePeers {
			pool.ReturnPeer(peer, false)
		}
		activePeers = make(map[redwood.Peer]struct{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		peer, err := pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Contains(t, peers[:2], peer)

		peer, err = pool.GetPeer(ctx)
		require.NoError(t, err)
		require.NotNil(t, peer)
		require.Contains(t, peers[:2], peer)
	})

	t.Run("when the providers channel closes, the pool requests a new one", func(t *testing.T) {
		oldChPeers := chPeers
		chPeers = make(chan redwood.Peer)
		close(oldChPeers)
		require.Eventually(t, func() bool { return atomic.LoadUint32(&numProvidersRequests) == 2 }, 1*time.Second, 10*time.Millisecond)
	})
}

func TestPeerPool_Integration(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		i := i
		t.Run("maintains invariants under randomized conditions", func(t *testing.T) {
			t.Parallel()

			rand.Seed(time.Now().UTC().Unix() + int64(i))

			var (
				chPeers   chan redwood.Peer
				chPeersMu sync.Mutex

				concurrentConns = rand.Intn(10) + 1
				numPeers        = rand.Intn(concurrentConns+3) + 1
				numGets         = rand.Intn(10) + 1
				chDone          = make(chan struct{})
				activePeers     sync.Map
				numActive       int32
				strickenPeers   sync.Map
				numStricken     int32
			)

			fmt.Println("concurrentConns:", concurrentConns)
			fmt.Println("numPeers:", numPeers)
			fmt.Println("numGets:", numGets)

			chPeersMu.Lock()

			pool := redwood.NewPeerPool(uint64(concurrentConns), func(ctx context.Context) (<-chan redwood.Peer, error) {
				defer chPeersMu.Unlock()
				chPeers = make(chan redwood.Peer)
				return chPeers, nil
			})
			defer pool.Close()

			wg := utils.NewWaitGroupChan(context.TODO())
			wg.Add(1)

			// Occasionally close chPeers
			go func() {
				for {
					if rand.Intn(10) < 2 {
						chPeersMu.Lock()
						close(chPeers)
					}
					time.Sleep(500 * time.Millisecond)

					select {
					case <-wg.Wait():
						return
					default:
					}
				}
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numPeers; j++ {
					peer := new(mocks.Peer)
					peer.On("DialInfo").Return(redwood.PeerDialInfo{TransportName: testutils.RandomString(t, 5), DialAddr: testutils.RandomString(t, 5)}).Maybe()
					peer.On("Ready").Return(true).Maybe()
					peer.On("Addresses").Return([]types.Address{testutils.RandomAddress(t)}).Maybe()
					peer.On("Close").Return(nil).Maybe()

					var abort bool
					func() {
						chPeersMu.Lock()
						defer chPeersMu.Unlock()

						select {
						case <-chDone:
							abort = true
						case chPeers <- peer:
						}
					}()
					if abort {
						return
					}
					time.Sleep(testutils.RandomDuration(t, 500*time.Millisecond))
				}
			}()

			ctx, cancel := utils.CombinedContext(chDone)
			defer cancel()

			// This semaphore prevents the test from trying to return a peer when none are currently active
			sem := semaphore.NewWeighted(int64(numGets))
			err := sem.Acquire(ctx, int64(numGets))
			require.NoError(t, err)

			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numGets; j++ {
					ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
					defer cancel()

					peer, err := pool.GetPeer(ctx)
					if int32(numPeers)-atomic.LoadInt32(&numStricken) <= 0 {
						require.Error(t, err)
						require.Nil(t, peer)
					} else {
						require.NoError(t, err)
						require.NotNil(t, peer)

						// Invariant: there should never be more active peers than `concurrentConns`
						// require.LessOrEqual(t, int(atomic.AddInt32(&numActive, 1)), concurrentConns)

						// Invariant: stricken peers are never returned
						_, exists := strickenPeers.Load(peer)
						require.False(t, exists)

						// Invariant: fetched peers' state should be `InUse`
						peersAfter := pool.CopyPeers()
						require.Equal(t, redwood.PeerState_InUse, peersAfter[peer.DialInfo()].State())

						activePeers.Store(peer, struct{}{})
						sem.Release(1)
					}
					time.Sleep(testutils.RandomDuration(t, 500*time.Millisecond))
				}
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numGets; j++ {
					if atomic.LoadInt32(&numStricken) == int32(numPeers) {
						continue
					}

					err := sem.Acquire(ctx, 1)
					require.NoError(t, err)

					var peer redwood.Peer
					activePeers.Range(func(key, val interface{}) bool {
						peer = key.(redwood.Peer)
						return false
					})
					require.NotNil(t, peer)

					// Invariant: fetched peers' state should be `InUse`
					peersBefore := pool.CopyPeers()
					require.Equal(t, redwood.PeerState_InUse, peersBefore[peer.DialInfo()].State())

					atomic.AddInt32(&numActive, -1)
					strike := rand.Intn(10) < 2
					pool.ReturnPeer(peer, strike)
					peersAfter := pool.CopyPeers()
					if strike {
						atomic.AddInt32(&numStricken, 1)
						// Invariant: stricken peers' state should be `Strike`
						require.Equal(t, redwood.PeerState_Strike, peersAfter[peer.DialInfo()].State())
						strickenPeers.Store(peer, struct{}{})
					} else {
						// Invariant: returned peers' state should be `Unknown`
						require.Equal(t, redwood.PeerState_Unknown, peersAfter[peer.DialInfo()].State())
					}

					activePeers.Delete(peer)
					time.Sleep(testutils.RandomDuration(t, 500*time.Millisecond))
				}
			}()

			wg.Done()

			<-wg.Wait()
			cancel()
		})
	}
}
