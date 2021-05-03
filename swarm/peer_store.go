package swarm

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"redwood.dev/crypto"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type PeerStore interface {
	AddDialInfos(dialInfos []PeerDialInfo)
	AddVerifiedCredentials(dialInfo PeerDialInfo, address types.Address, sigpubkey crypto.SigningPublicKey, encpubkey crypto.EncryptingPublicKey)
	RemovePeers(dialInfos []PeerDialInfo) error
	UnverifiedPeers() []*peerDetails
	Peers() []*peerDetails
	AllDialInfos() []PeerDialInfo
	PeerWithDialInfo(dialInfo PeerDialInfo) *peerDetails
	PeersWithAddress(address types.Address) []*peerDetails
	PeersFromTransport(transportName string) []*peerDetails
	PeersFromTransportWithAddress(transportName string, address types.Address) []*peerDetails
	PeersServingStateURI(stateURI string) []*peerDetails
	IsKnownPeer(dialInfo PeerDialInfo) bool
	OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo))
}

type peerStore struct {
	log.Logger

	state            *state.DBTree
	muPeers          sync.RWMutex
	peers            map[PeerDialInfo]*peerDetails
	peersWithAddress map[types.Address]map[PeerDialInfo]*peerDetails
	unverifiedPeers  map[PeerDialInfo]struct{}

	onNewUnverifiedPeer func(dialInfo PeerDialInfo)
}

type PeerDialInfo struct {
	TransportName string
	DialAddr      string
}

var (
	ErrNotReady = errors.New("not ready")
)

func NewPeerStore(state *state.DBTree) *peerStore {
	s := &peerStore{
		Logger:           log.NewLogger("peerstore"),
		state:            state,
		peers:            make(map[PeerDialInfo]*peerDetails),
		peersWithAddress: make(map[types.Address]map[PeerDialInfo]*peerDetails),
		unverifiedPeers:  make(map[PeerDialInfo]struct{}),
	}

	pds, err := s.fetchAllPeerDetails()
	if err != nil {
		s.Warnf("could not fetch stored peer details from DB: %v", err)
	} else {
		for _, pd := range pds {
			s.peers[pd.dialInfo] = pd

			if len(pd.addresses) > 0 {
				for addr := range pd.addresses {
					if _, exists := s.peersWithAddress[addr]; !exists {
						s.peersWithAddress[addr] = make(map[PeerDialInfo]*peerDetails)
					}
					s.peersWithAddress[addr][pd.dialInfo] = pd
				}

			} else {
				s.unverifiedPeers[pd.dialInfo] = struct{}{}
			}
		}
	}

	return s
}

func (s *peerStore) Peers() []*peerDetails {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	var pds []*peerDetails
	for _, pd := range s.peers {
		pds = append(pds, pd)
	}
	return pds
}

func (s *peerStore) OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo)) {
	s.onNewUnverifiedPeer = fn
}

func (s *peerStore) AddDialInfos(dialInfos []PeerDialInfo) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for _, dialInfo := range dialInfos {
		if dialInfo.DialAddr == "" {
			continue
		}

		_, exists := s.peers[dialInfo]
		if !exists {
			peerDetails := newPeerDetails(s, dialInfo)

			err := s.savePeerDetails(peerDetails)
			if err != nil {
				s.Warnf("could not save modifications to peerstore DB: %v", err)
			}

			s.peers[dialInfo] = peerDetails
			s.unverifiedPeers[dialInfo] = struct{}{}
			s.onNewUnverifiedPeer(dialInfo)
		}
	}
}

func (s *peerStore) AddVerifiedCredentials(
	dialInfo PeerDialInfo,
	address types.Address,
	sigpubkey crypto.SigningPublicKey,
	encpubkey crypto.EncryptingPublicKey,
) {
	if address.IsZero() {
		panic("cannot add verified peer without credentials")
	}

	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	var pd *peerDetails
	if _, exists := s.peersWithAddress[address]; exists {
		pd = s.peersWithAddress[address][dialInfo]
	}
	if pd == nil {
		if p, exists := s.peers[dialInfo]; exists {
			pd = p
		}
	}
	if pd == nil {
		pd = newPeerDetails(s, dialInfo)
	}

	pd.peerStore = s
	if pd.addresses == nil {
		pd.addresses = utils.NewAddressSet(nil)
	}
	pd.addresses.Add(address)

	if pd.sigpubkeys == nil {
		pd.sigpubkeys = make(map[types.Address]crypto.SigningPublicKey)
	}
	if pd.encpubkeys == nil {
		pd.encpubkeys = make(map[types.Address]crypto.EncryptingPublicKey)
	}
	if sigpubkey != nil {
		pd.sigpubkeys[address] = sigpubkey
	}
	if encpubkey != nil {
		pd.encpubkeys[address] = encpubkey
	}

	if _, exists := s.peersWithAddress[address]; !exists {
		s.peersWithAddress[address] = make(map[PeerDialInfo]*peerDetails)
	}
	s.peersWithAddress[address][dialInfo] = pd

	if dialInfo.DialAddr != "" {
		s.peers[dialInfo] = pd
	}

	delete(s.unverifiedPeers, pd.dialInfo)

	err := s.savePeerDetails(pd)
	if err != nil {
		s.Warnf("could not save modifications to peerstore DB: %v", err)
	}
}

func (s *peerStore) RemovePeers(dialInfos []PeerDialInfo) error {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for _, dialInfo := range dialInfos {
		delete(s.peers, dialInfo)
		delete(s.unverifiedPeers, dialInfo)
		for addr := range s.peersWithAddress {
			delete(s.peersWithAddress[addr], dialInfo)
		}
	}
	return s.deletePeers(dialInfos)
}

func (s *peerStore) PeerWithDialInfo(dialInfo PeerDialInfo) *peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	if dialInfo.DialAddr == "" {
		return nil
	}
	return s.peers[dialInfo]
}

func (s *peerStore) UnverifiedPeers() []*peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	unverifiedPeers := make([]*peerDetails, len(s.unverifiedPeers))
	i := 0
	for dialInfo := range s.unverifiedPeers {
		unverifiedPeers[i] = s.peers[dialInfo]
		i++
	}
	return unverifiedPeers
}

func (s *peerStore) AllDialInfos() []PeerDialInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var dialInfos []PeerDialInfo
	for di := range s.peers {
		dialInfos = append(dialInfos, di)
	}
	return dialInfos
}

func (s *peerStore) PeersWithAddress(address types.Address) []*peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*peerDetails
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peerDetails := range s.peersWithAddress[address] {
			peers = append(peers, peerDetails)
		}
	}
	return peers
}

func (s *peerStore) PeersFromTransport(transportName string) []*peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*peerDetails
	for dialInfo, peerDetails := range s.peers {
		if dialInfo.TransportName == transportName {
			peers = append(peers, peerDetails)
		}
	}
	return peers
}

func (s *peerStore) PeersFromTransportWithAddress(transport string, address types.Address) []*peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*peerDetails
	if _, exists := s.peersWithAddress[address]; exists {
		for dialInfo, peerDetails := range s.peersWithAddress[address] {
			if dialInfo.TransportName == transport {
				peers = append(peers, peerDetails)
			}
		}
	}
	return peers
}

func (s *peerStore) PeersServingStateURI(stateURI string) []*peerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*peerDetails
	for _, peerDetails := range s.peers {
		if peerDetails.stateURIs.Contains(stateURI) {
			peers = append(peers, peerDetails)
		}
	}
	return peers
}

func (s *peerStore) IsKnownPeer(dialInfo PeerDialInfo) bool {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	_, exists := s.peers[dialInfo]
	return exists
}

func (s *peerStore) dialInfoHash(dialInfo PeerDialInfo) string {
	return types.HashBytes([]byte(dialInfo.TransportName + ":" + dialInfo.DialAddr)).Hex()
}

func (s *peerStore) fetchAllPeerDetails() ([]*peerDetails, error) {
	node := s.state.State(false)
	defer node.Close()

	keypath := state.Keypath("peers")

	var pdCodecs map[string]peerDetailsCodec
	err := node.NodeAt(keypath, nil).Scan(&pdCodecs)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch peer details")
	}

	var decoded []*peerDetails
	for _, codec := range pdCodecs {
		pd, err := s.peerDetailsCodecToPeerDetails(codec)
		if err != nil {
			return nil, err
		}
		decoded = append(decoded, pd)
	}
	return decoded, nil
}

func (s *peerStore) fetchPeerDetails(dialInfo PeerDialInfo) (*peerDetails, error) {
	node := s.state.State(false)
	defer node.Close()

	dialInfoHash := s.dialInfoHash(dialInfo)
	peerKeypath := state.Keypath("peers").Pushs(dialInfoHash)

	var pd peerDetailsCodec
	err := node.NodeAt(peerKeypath, nil).Scan(&pd)
	if err != nil {
		return nil, errors.Wrap(err, "could not fetch peer details")
	}
	return s.peerDetailsCodecToPeerDetails(pd)
}

func (s *peerStore) peerDetailsCodecToPeerDetails(pd peerDetailsCodec) (*peerDetails, error) {
	sigpubkeys := make(map[types.Address]crypto.SigningPublicKey, len(pd.Sigpubkeys))
	encpubkeys := make(map[types.Address]crypto.EncryptingPublicKey, len(pd.Encpubkeys))
	for addrStr, bytes := range pd.Sigpubkeys {
		addr, err := types.AddressFromHex(addrStr)
		if err != nil {
			return nil, err
		}
		sigpubkeys[addr], err = crypto.SigningPublicKeyFromBytes(bytes)
		if err != nil {
			return nil, err
		}
	}
	for addrStr, bytes := range pd.Encpubkeys {
		addr, err := types.AddressFromHex(addrStr)
		if err != nil {
			return nil, err
		}
		encpubkeys[addr] = crypto.EncryptingPublicKeyFromBytes(bytes)
	}

	return &peerDetails{
		peerStore:   s,
		dialInfo:    pd.DialInfo,
		addresses:   utils.NewAddressSet(pd.Addresses),
		sigpubkeys:  sigpubkeys,
		encpubkeys:  encpubkeys,
		stateURIs:   utils.NewStringSet(pd.StateURIs),
		lastContact: time.Unix(int64(pd.LastContact), 0),
		lastFailure: time.Unix(int64(pd.LastFailure), 0),
		failures:    pd.Failures,
		backoff:     utils.ExponentialBackoff{Min: 10 * time.Second, Max: 3 * time.Minute},
	}, nil
}

func (s *peerStore) savePeerDetails(peerDetails *peerDetails) error {
	node := s.state.State(true)
	defer node.Close()

	dialInfoHash := s.dialInfoHash(peerDetails.dialInfo)
	peerKeypath := state.Keypath("peers").Pushs(dialInfoHash)

	pdc := &peerDetailsCodec{
		DialInfo:    peerDetails.dialInfo,
		Addresses:   peerDetails.addresses.Slice(),
		StateURIs:   peerDetails.stateURIs.Slice(),
		LastContact: uint64(peerDetails.lastContact.UTC().Unix()),
		LastFailure: uint64(peerDetails.lastFailure.UTC().Unix()),
		Failures:    peerDetails.failures,
	}
	pdc.Sigpubkeys = make(map[string][]byte, len(peerDetails.sigpubkeys))
	for addr, key := range peerDetails.sigpubkeys {
		pdc.Sigpubkeys[addr.Hex()] = key.Bytes()
	}
	pdc.Encpubkeys = make(map[string][]byte, len(peerDetails.encpubkeys))
	for addr, key := range peerDetails.encpubkeys {
		pdc.Encpubkeys[addr.Hex()] = key.Bytes()
	}

	err := node.Set(peerKeypath, nil, pdc)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *peerStore) deletePeers(dialInfos []PeerDialInfo) error {
	node := s.state.State(true)
	defer node.Close()

	for _, dialInfo := range dialInfos {
		dialInfoHash := s.dialInfoHash(dialInfo)
		peerKeypath := state.Keypath("peers").Pushs(dialInfoHash)
		err := node.Delete(peerKeypath, nil)
		if err != nil {
			return err
		}
	}
	return node.Save()
}

type PeerDetails interface {
	Addresses() []types.Address
	DialInfo() PeerDialInfo
	PublicKeys(addr types.Address) (crypto.SigningPublicKey, crypto.EncryptingPublicKey)
	AddStateURI(stateURI string)
	RemoveStateURI(stateURI string)
	StateURIs() utils.StringSet

	UpdateConnStats(success bool)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() uint64
	Ready() bool
	RemainingBackoff() time.Duration
}

type peerDetails struct {
	peerStore   *peerStore
	dialInfo    PeerDialInfo
	addresses   utils.AddressSet
	sigpubkeys  map[types.Address]crypto.SigningPublicKey
	encpubkeys  map[types.Address]crypto.EncryptingPublicKey
	stateURIs   utils.StringSet
	lastContact time.Time
	lastFailure time.Time
	failures    uint64
	backoff     utils.ExponentialBackoff
}

type peerDetailsCodec struct {
	DialInfo    PeerDialInfo      `tree:"dialInfo"`
	Addresses   []types.Address   `tree:"address"`
	Sigpubkeys  map[string][]byte `tree:"sigpubkey"`
	Encpubkeys  map[string][]byte `tree:"encpubkey"`
	StateURIs   []string          `tree:"stateURIs"`
	LastContact uint64            `tree:"lastContact"`
	LastFailure uint64            `tree:"lastFailure"`
	Failures    uint64            `tree:"failures"`
}

func newPeerDetails(peerStore *peerStore, dialInfo PeerDialInfo) *peerDetails {
	return &peerDetails{
		peerStore: peerStore,
		dialInfo:  dialInfo,
		stateURIs: utils.NewStringSet(nil),
		backoff:   utils.ExponentialBackoff{Min: 10 * time.Second, Max: 3 * time.Minute},
	}
}

func (p *peerDetails) Addresses() []types.Address {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.addresses.Slice()
}

func (p *peerDetails) PublicKeys(addr types.Address) (crypto.SigningPublicKey, crypto.EncryptingPublicKey) {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.sigpubkeys[addr], p.encpubkeys[addr]
}

func (p *peerDetails) DialInfo() PeerDialInfo {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.dialInfo
}

func (p *peerDetails) AddStateURI(stateURI string) {
	p.peerStore.muPeers.Lock()
	defer p.peerStore.muPeers.Unlock()
	p.stateURIs.Add(stateURI)
	p.peerStore.savePeerDetails(p)
}

func (p *peerDetails) RemoveStateURI(stateURI string) {
	p.peerStore.muPeers.Lock()
	defer p.peerStore.muPeers.Unlock()
	p.stateURIs.Remove(stateURI)
	p.peerStore.savePeerDetails(p)
}

func (p *peerDetails) StateURIs() utils.StringSet {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.stateURIs.Copy()
}

func (p *peerDetails) UpdateConnStats(success bool) {
	p.peerStore.muPeers.Lock()
	defer p.peerStore.muPeers.Unlock()
	now := time.Now()
	if success {
		p.lastContact = now
		p.failures = 0
	} else {
		p.lastContact = now
		p.lastFailure = now
		p.failures++
		p.backoff.Next()
	}
	p.peerStore.savePeerDetails(p)
}

func (p *peerDetails) LastContact() time.Time {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.lastContact
}

func (p *peerDetails) LastFailure() time.Time {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.lastFailure
}

func (p *peerDetails) Failures() uint64 {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.failures
}

// @@TODO: this should be configurable and probably not handled here
func (p *peerDetails) Ready() bool {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	ready, _ := p.backoff.Ready()
	return ready
}

func (p *peerDetails) RemainingBackoff() time.Duration {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	_, remaining := p.backoff.Ready()
	return remaining
}

// @@TODO: remove this and use transport-defined unique IDs to
// identify peers so that *everything* is tracked
type ephemeralPeerDetails peerDetails

func NewEphemeralPeerDetails(dialInfo PeerDialInfo) *ephemeralPeerDetails {
	return &ephemeralPeerDetails{
		dialInfo: dialInfo,
		backoff:  utils.ExponentialBackoff{Min: 10 * time.Second, Max: 3 * time.Minute},
	}
}

func (p *ephemeralPeerDetails) Addresses() []types.Address { return nil }
func (p *ephemeralPeerDetails) PublicKeys(addr types.Address) (crypto.SigningPublicKey, crypto.EncryptingPublicKey) {
	return nil, nil
}
func (p *ephemeralPeerDetails) DialInfo() PeerDialInfo         { return p.dialInfo }
func (p *ephemeralPeerDetails) AddStateURI(stateURI string)    {}
func (p *ephemeralPeerDetails) RemoveStateURI(stateURI string) {}
func (p *ephemeralPeerDetails) StateURIs() utils.StringSet     { return nil }
func (p *ephemeralPeerDetails) UpdateConnStats(success bool) {
	now := time.Now()
	if success {
		p.lastContact = now
		p.failures = 0
	} else {
		p.lastContact = now
		p.lastFailure = now
		p.failures++
		p.backoff.Next()
	}
}

func (p *ephemeralPeerDetails) LastContact() time.Time {
	return p.lastContact
}

func (p *ephemeralPeerDetails) LastFailure() time.Time {
	return p.lastFailure
}

func (p *ephemeralPeerDetails) Failures() uint64 {
	return p.failures
}

func (p *ephemeralPeerDetails) Ready() bool {
	ready, _ := p.backoff.Ready()
	return ready
}

func (p *ephemeralPeerDetails) RemainingBackoff() time.Duration {
	_, remaining := p.backoff.Ready()
	return remaining
}
