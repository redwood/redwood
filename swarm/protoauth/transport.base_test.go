package protoauth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/errors"
	"redwood.dev/internal/testutils"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoauth/mocks"
)

func TestBaseAuthTransport_ChallengeIdentity(t *testing.T) {
	t.Parallel()

	var transport protoauth.BaseAuthTransport

	expectedPeerConn := new(mocks.AuthPeerConn)
	expectedErr := errors.New("whoops")
	expectedChallengeMsgBytes, err := protoauth.GenerateChallengeMsg()
	require.NoError(t, err)
	expectedChallengeMsg := protoauth.ChallengeMsg(expectedChallengeMsgBytes)

	callback1 := testutils.NewAwaiter()
	callback2 := testutils.NewAwaiter()

	transport.OnChallengeIdentity(func(challengeMsg protoauth.ChallengeMsg, peerConn protoauth.AuthPeerConn) error {
		require.Equal(t, expectedChallengeMsg, challengeMsg)
		require.Equal(t, expectedPeerConn, peerConn)
		callback1.ItHappened()
		return nil
	})

	transport.OnChallengeIdentity(func(challengeMsg protoauth.ChallengeMsg, peerConn protoauth.AuthPeerConn) error {
		require.Equal(t, expectedChallengeMsg, challengeMsg)
		require.Equal(t, expectedPeerConn, peerConn)
		callback2.ItHappened()
		return expectedErr
	})

	err = transport.HandleChallengeIdentity(expectedChallengeMsg, expectedPeerConn)
	require.Error(t, err)
	require.Contains(t, err.Error(), expectedErr.Error())

	callback1.AwaitOrFail(t, 1*time.Second)
	callback2.AwaitOrFail(t, 1*time.Second)
}
