package protoauth_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	identitymocks "redwood.dev/identity/mocks"
	"redwood.dev/internal/testutils"
	"redwood.dev/swarm"
	swarmmocks "redwood.dev/swarm/mocks"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoauth/mocks"
	"redwood.dev/types"
)

func TestAuthProtocol_PeersClaimingAddress(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		addr1 := testutils.RandomAddress(t)

		chHttp := make(chan protoauth.AuthPeerConn)
		httpPeers := []protoauth.AuthPeerConn{new(mocks.AuthPeerConn), new(mocks.AuthPeerConn), new(mocks.AuthPeerConn)}
		go func() {
			defer close(chHttp)
			for _, peer := range httpPeers {
				chHttp <- peer
			}
		}()

		chLibp2p := make(chan protoauth.AuthPeerConn)
		libp2pPeers := []protoauth.AuthPeerConn{new(mocks.AuthPeerConn), new(mocks.AuthPeerConn), new(mocks.AuthPeerConn)}
		go func() {
			defer close(chLibp2p)
			for _, peer := range libp2pPeers {
				chLibp2p <- peer
			}
		}()

		http.On("PeersClaimingAddress", mock.Anything, addr1).Return((<-chan protoauth.AuthPeerConn)(chHttp), nil)
		libp2p.On("PeersClaimingAddress", mock.Anything, addr1).Return((<-chan protoauth.AuthPeerConn)(chLibp2p), nil)

		var recvd []protoauth.AuthPeerConn
		for peer := range proto.PeersClaimingAddress(context.Background(), addr1) {
			recvd = append(recvd, peer)
		}

		expected := append(httpPeers, libp2pPeers...)
		for _, x := range expected {
			require.Contains(t, recvd, x)
		}

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
	})

	t.Run("terminates when context is canceled", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		addr1 := testutils.RandomAddress(t)

		chDone := make(chan struct{})
		defer close(chDone)

		chHttp := make(chan protoauth.AuthPeerConn)
		go func() {
			for {
				select {
				case chHttp <- new(mocks.AuthPeerConn):
				case <-chDone:
					return
				}
			}
		}()

		chLibp2p := make(chan protoauth.AuthPeerConn)
		go func() {
			for {
				select {
				case chLibp2p <- new(mocks.AuthPeerConn):
				case <-chDone:
					return
				}
			}
		}()

		http.On("PeersClaimingAddress", mock.Anything, addr1).Return((<-chan protoauth.AuthPeerConn)(chHttp), nil)
		libp2p.On("PeersClaimingAddress", mock.Anything, addr1).Return((<-chan protoauth.AuthPeerConn)(chLibp2p), nil)

		ctx, cancel := context.WithCancel(context.Background())

		ch := proto.PeersClaimingAddress(ctx, addr1)
		select {
		case _, open := <-ch:
			require.True(t, open)
		case <-time.After(1 * time.Second):
			t.Fatal("could not receive")
		}

		select {
		case _, open := <-ch:
			require.True(t, open)
		case <-time.After(1 * time.Second):
			t.Fatal("could not receive")
		}

		cancel()

		// Give it a moment to terminate
		time.Sleep(1 * time.Second)

		select {
		case _, open := <-ch:
			require.False(t, open)
		case <-time.After(1 * time.Second):
			t.Fatal("could not receive")
		}

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
	})
}

func TestAuthProtocol_ChallengePeerIdentity(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
			peerConn         = new(mocks.AuthPeerConn)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		identity1 := testutils.RandomIdentity(t)
		identity2 := testutils.RandomIdentity(t)
		dialInfo := swarm.PeerDialInfo{"foo", "bar"}

		peerConn.On("EnsureConnected", mock.Anything).Return(nil).Once()
		peerConn.On("DialInfo").Return(dialInfo)

		var challengeMsg protoauth.ChallengeMsg
		peerConn.On("ChallengeIdentity", mock.Anything).Run(func(args mock.Arguments) {
			challengeMsg = args.Get(0).(protoauth.ChallengeMsg)
		}).Return(nil)

		call := peerConn.On("ReceiveChallengeIdentityResponse").Once()
		call.Run(func(args mock.Arguments) {
			sig1, err := identity1.SignHash(types.HashBytes(challengeMsg))
			require.NoError(t, err)
			sig2, err := identity2.SignHash(types.HashBytes(challengeMsg))
			require.NoError(t, err)
			resp := []protoauth.ChallengeIdentityResponse{
				{
					Signature:           sig1,
					EncryptingPublicKey: identity1.Encrypting.EncryptingPublicKey.Bytes(),
				},
				{
					Signature:           sig2,
					EncryptingPublicKey: identity2.Encrypting.EncryptingPublicKey.Bytes(),
				},
			}
			call.ReturnArguments = mock.Arguments{resp, nil}
		})

		peerStore.On("AddVerifiedCredentials", dialInfo, identity1.Address(), identity1.Signing.SigningPublicKey, identity1.Encrypting.EncryptingPublicKey).Once()
		peerStore.On("AddVerifiedCredentials", dialInfo, identity2.Address(), identity2.Signing.SigningPublicKey, identity2.Encrypting.EncryptingPublicKey).Once()

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.NoError(t, err)

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})

	t.Run("error on EnsureConnected", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
			peerConn         = new(mocks.AuthPeerConn)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("EnsureConnected", mock.Anything).Return(expectedErr).Once()

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.Equal(t, expectedErr, errors.Cause(err))

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})

	t.Run("error on ChallengeIdentity", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
			peerConn         = new(mocks.AuthPeerConn)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("EnsureConnected", mock.Anything).Return(nil).Once()
		peerConn.On("ChallengeIdentity", mock.Anything).Return(expectedErr)

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.Equal(t, expectedErr, errors.Cause(err))

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})

	t.Run("error on ReceiveChallengeIdentityResponse", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
			peerConn         = new(mocks.AuthPeerConn)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("EnsureConnected", mock.Anything).Return(nil).Once()
		peerConn.On("ChallengeIdentity", mock.Anything).Return(nil)
		peerConn.On("ReceiveChallengeIdentityResponse").Return(nil, expectedErr).Once()

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.Equal(t, expectedErr, errors.Cause(err))

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})

	t.Run("bad signature", func(t *testing.T) {
		t.Parallel()

		var (
			http             = new(mocks.AuthTransport)
			libp2p           = new(mocks.AuthTransport)
			notAuthTransport = new(swarmmocks.Transport)
			transports       = []swarm.Transport{http, libp2p, notAuthTransport}
			keyStore         = new(identitymocks.KeyStore)
			peerStore        = new(swarmmocks.PeerStore)
			peerConn         = new(mocks.AuthPeerConn)
		)

		http.On("Name").Return("http")
		http.On("OnChallengeIdentity", mock.Anything).Return()
		libp2p.On("Name").Return("libp2p")
		libp2p.On("OnChallengeIdentity", mock.Anything).Return()

		peerStore.On("OnNewUnverifiedPeer", mock.Anything).Return()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		identity1 := testutils.RandomIdentity(t)
		identity2 := testutils.RandomIdentity(t)
		dialInfo := swarm.PeerDialInfo{"foo", "bar"}

		peerConn.On("EnsureConnected", mock.Anything).Return(nil).Once()
		peerConn.On("DialInfo").Return(dialInfo)

		var challengeMsg protoauth.ChallengeMsg
		peerConn.On("ChallengeIdentity", mock.Anything).Run(func(args mock.Arguments) {
			challengeMsg = args.Get(0).(protoauth.ChallengeMsg)
		}).Return(nil)

		call := peerConn.On("ReceiveChallengeIdentityResponse").Once()
		call.Run(func(args mock.Arguments) {
			sig1, err := identity1.SignHash(types.HashBytes(challengeMsg))
			require.NoError(t, err)
			sig2, err := identity2.SignHash(types.HashBytes(challengeMsg))
			require.NoError(t, err)

			// Mess with the 2nd signature
			badSig2 := sig2
			for bytes.Equal(sig2, badSig2) {
				badSig2 = testutils.RandomBytes(t, len(sig2))
			}

			resp := []protoauth.ChallengeIdentityResponse{
				{
					Signature:           sig1,
					EncryptingPublicKey: identity1.Encrypting.EncryptingPublicKey.Bytes(),
				},
				{
					Signature:           badSig2,
					EncryptingPublicKey: identity2.Encrypting.EncryptingPublicKey.Bytes(),
				},
			}
			call.ReturnArguments = mock.Arguments{resp, nil}
		})

		peerStore.On("AddVerifiedCredentials", dialInfo, identity1.Address(), identity1.Signing.SigningPublicKey, identity1.Encrypting.EncryptingPublicKey).Once()

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.Error(t, err)

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})
}
