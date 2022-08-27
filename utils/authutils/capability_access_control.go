package authutils

import (
	"crypto/rand"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/types"
	. "redwood.dev/utils/generics"
)

type UcanAccessControl[C comparable] struct {
	jwtSecret             []byte
	jwtExpiry             time.Duration
	pendingAuthorizations SyncSet[string]
	defaultCapabilities   SyncSet[C]
	capabilitiesByAddress SyncMap[types.Address, SyncSet[C]]
}

func NewUcanAccessControl[C comparable](
	jwtSecret []byte,
	jwtExpiry time.Duration,
	defaultCapabilities Set[C],
) *UcanAccessControl[C] {
	if len(jwtSecret) == 0 {
		jwtSecret = make([]byte, 64)
		_, err := rand.Read(jwtSecret)
		if err != nil {
			panic(err)
		}
	}
	return &UcanAccessControl[C]{
		jwtSecret:             jwtSecret,
		jwtExpiry:             jwtExpiry,
		pendingAuthorizations: NewSyncSet[string](nil),
		defaultCapabilities:   NewSyncSet(defaultCapabilities.Slice()),
		capabilitiesByAddress: NewSyncMap[types.Address, SyncSet[C]](),
	}
}

func (ac *UcanAccessControl[C]) SetCapabilities(addr types.Address, capabilities []C) {
	ac.capabilitiesByAddress.Set(addr, NewSyncSet(capabilities))
}

func (ac *UcanAccessControl[C]) SetDefaultCapabilities(capabilities []C) {
	ac.defaultCapabilities.Replace(capabilities)
}

func (ac *UcanAccessControl[C]) GenerateChallenge() (crypto.ChallengeMsg, error) {
	challenge, err := crypto.GenerateChallengeMsg()
	if err != nil {
		return nil, err
	}
	ac.pendingAuthorizations.Add(challenge.String())
	return challenge, nil
}

type ChallengeSignature struct {
	Challenge crypto.ChallengeMsg
	Signature crypto.Signature
}

func (ac *UcanAccessControl[C]) RespondChallenge(responses []ChallengeSignature) (Ucan, string, error) {
	addrs := NewSet[types.Address](nil)
	// capabilities := NewSet[C](nil)
	for _, response := range responses {
		if !ac.pendingAuthorizations.Contains(response.Challenge.String()) {
			continue
		}
		sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(response.Challenge.Bytes()), response.Signature)
		if err != nil {
			continue
		}
		ac.pendingAuthorizations.Remove(response.Challenge.String()) // @@TODO: expiration/garbage collection for failed auths
		addrs.Add(sigpubkey.Address())

		// moreCaps, _ := ac.capabilitiesByAddress.Get(sigpubkey.Address())
		// capabilities = capabilities.Union(moreCaps.Unwrap())
	}

	ucan := Ucan{
		Addresses: addrs,
		// Capabilities: ac.defaultCapabilities.Unwrap().Union(capabilities),
		IssuedAt: time.Now(),
	}
	ucanStr, err := ucan.SignedString(ac.jwtSecret)
	if err != nil {
		return Ucan{}, "", err
	}
	return ucan, ucanStr, nil
}

// func (ac *UcanAccessControl[C]) UserHasCapability(addr types.Address, capability C) bool {
// 	capabilities, exists := ac.capabilitiesByAddress.Get(addr)
// 	if !exists {
// 		return ac.defaultCapabilities.Contains(capability)
// 	}
// 	return capabilities.Contains(capability)
// }

var ErrUcanExpired = errors.New("ucan expired")

func (ac *UcanAccessControl[C]) UserHasCapabilityByJWT(ucanStr string, capability C) (bool, error) {
	if ac.defaultCapabilities.Contains(capability) {
		return true, nil
	}

	var ucan Ucan
	err := ucan.Parse(ucanStr, ac.jwtSecret)
	if err != nil {
		return false, err
	} else if ac.jwtExpiry > 0 && time.Now().Sub(ucan.IssuedAt) > ac.jwtExpiry {
		return false, ErrUcanExpired
	}

	for addr := range ucan.Addresses {
		caps, _ := ac.capabilitiesByAddress.Get(addr)
		if caps.Contains(capability) {
			return true, nil
		}
	}
	return false, nil
}

func (ac *UcanAccessControl[C]) UserCapabilities(addr types.Address) Set[C] {
	capabilities, _ := ac.capabilitiesByAddress.Get(addr)
	return capabilities.Unwrap().Union(ac.defaultCapabilities.Unwrap())
}

func (ac *UcanAccessControl[C]) UserCapabilitiesByJWT(ucanStr string) (Set[C], error) {
	var ucan Ucan
	err := ucan.Parse(ucanStr, ac.jwtSecret)
	if err != nil {
		return nil, err
	}

	capabilities := ac.defaultCapabilities.Unwrap()
	for addr := range ucan.Addresses {
		caps, _ := ac.capabilitiesByAddress.Get(addr)
		capabilities = capabilities.Union(caps.Unwrap())
	}
	return capabilities, nil
}
