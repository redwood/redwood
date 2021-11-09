package protoauth_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"redwood.dev/errors"
	identitymocks "redwood.dev/identity/mocks"
	"redwood.dev/internal/testutils"
	"redwood.dev/swarm"
	swarmmocks "redwood.dev/swarm/mocks"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoauth/mocks"
	"redwood.dev/types"
)

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
		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		identity1 := testutils.RandomIdentity(t)
		identity2 := testutils.RandomIdentity(t)
		dialInfo := swarm.PeerDialInfo{"foo", "bar"}

		peerConn.On("Ready").Return(true).Maybe()
		peerConn.On("Dialable").Return(true).Maybe()
		peerConn.On("DeviceUniqueID").Return("foo").Maybe()
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
					Signature:     sig1,
					AsymEncPubkey: identity1.AsymEncKeypair.AsymEncPubkey.Bytes(),
				},
				{
					Signature:     sig2,
					AsymEncPubkey: identity2.AsymEncKeypair.AsymEncPubkey.Bytes(),
				},
			}
			call.ReturnArguments = mock.Arguments{resp, nil}
		})

		peerStore.On("AddVerifiedCredentials", dialInfo, "foo", identity1.Address(), identity1.SigKeypair.SigningPublicKey, identity1.AsymEncKeypair.AsymEncPubkey).Return(nil).Once()
		peerStore.On("AddVerifiedCredentials", dialInfo, "foo", identity2.Address(), identity2.SigKeypair.SigningPublicKey, identity2.AsymEncKeypair.AsymEncPubkey).Return(nil).Once()

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
		peerStore.On("UnverifiedPeers", mock.Anything).Return(nil)

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("EnsureConnected", mock.Anything).Return(expectedErr).Once()
		peerConn.On("Ready").Return(true).Maybe()
		peerConn.On("Dialable").Return(true).Maybe()
		peerConn.On("DeviceUniqueID").Return("foo").Maybe()

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
		peerStore.On("UnverifiedPeers", mock.Anything).Return(nil)

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("Ready").Return(true).Maybe()
		peerConn.On("Dialable").Return(true).Maybe()
		peerConn.On("DeviceUniqueID").Return("foo").Maybe()
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
		peerStore.On("UnverifiedPeers", mock.Anything).Return(nil)

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		expectedErr := errors.New("")

		peerConn.On("Ready").Return(true).Maybe()
		peerConn.On("Dialable").Return(true).Maybe()
		peerConn.On("DeviceUniqueID").Return("foo").Maybe()
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
		peerStore.On("UnverifiedPeers", mock.Anything).Return(nil)

		proto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
		proto.Start()
		defer proto.Close()

		peerStore.On("AllDialInfos").Return(nil).Maybe()
		peerStore.On("UnverifiedPeers").Return(nil).Maybe()

		identity1 := testutils.RandomIdentity(t)
		identity2 := testutils.RandomIdentity(t)
		dialInfo := swarm.PeerDialInfo{"foo", "bar"}

		peerConn.On("Ready").Return(true).Maybe()
		peerConn.On("Dialable").Return(true).Maybe()
		peerConn.On("DeviceUniqueID").Return("foo").Maybe()
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
					Signature:     sig1,
					AsymEncPubkey: identity1.AsymEncKeypair.AsymEncPubkey.Bytes(),
				},
				{
					Signature:     badSig2,
					AsymEncPubkey: identity2.AsymEncKeypair.AsymEncPubkey.Bytes(),
				},
			}
			call.ReturnArguments = mock.Arguments{resp, nil}
		})

		peerStore.On("AddVerifiedCredentials", dialInfo, "foo", identity1.Address(), identity1.SigKeypair.SigningPublicKey, identity1.AsymEncKeypair.AsymEncPubkey).Return(nil).Once()

		err := proto.ChallengePeerIdentity(context.Background(), peerConn)
		require.Error(t, err)

		http.AssertExpectations(t)
		libp2p.AssertExpectations(t)
		peerStore.AssertExpectations(t)
		peerConn.AssertExpectations(t)
	})
}
