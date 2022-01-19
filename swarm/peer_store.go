package swarm

import (
	"bytes"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name PeerStore --output ./mocks/ --case=underscore
type PeerStore interface {
	AddDialInfo(dialInfo PeerDialInfo, deviceUniqueID string) PeerEndpoint
	AddVerifiedCredentials(dialInfo PeerDialInfo, deviceUniqueID string, address types.Address, sigpubkey *crypto.SigningPublicKey, encpubkey *crypto.AsymEncPubkey) PeerEndpoint
	RemovePeers(deviceUniqueIDs []string) error
	UnverifiedPeers() []PeerDialInfo
	VerifiedPeers() []PeerInfo
	Peers() []PeerInfo
	AllDialInfos() map[PeerDialInfo]struct{}
	PeerWithDeviceUniqueID(deviceUniqueID string) (PeerInfo, bool)
	PeerEndpoint(dialInfo PeerDialInfo) PeerEndpoint
	PeersWithAddress(address types.Address) []PeerInfo
	PeersFromTransport(transportName string) []PeerEndpoint
	PeersServingStateURI(stateURI string) []PeerInfo
	IsKnownPeer(dialInfo PeerDialInfo) bool
	OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo))
	OnNewVerifiedPeer(fn func(peer PeerInfo))

	DebugPrint()
}

type peerStore struct {
	log.Logger

	state                   *state.DBTree
	muPeers                 sync.RWMutex
	endpoints               map[PeerDialInfo]*endpoint
	peersWithAddress        map[types.Address]map[string]struct{}
	peersWithDeviceUniqueID map[string]*peerDetails
	unverifiedPeers         map[PeerDialInfo]struct{}

	newUnverifiedPeerListeners   []func(dialInfo PeerDialInfo)
	newUnverifiedPeerListenersMu sync.RWMutex
	newVerifiedPeerListeners     []func(peer PeerInfo)
	newVerifiedPeerListenersMu   sync.RWMutex
}

type PeerDialInfo struct {
	TransportName string
	DialAddr      string
}

func (pdi PeerDialInfo) ID() process.PoolUniqueID {
	return pdi.String()
}

func (pdi PeerDialInfo) String() string {
	return strings.Join([]string{pdi.TransportName, pdi.DialAddr}, " ")
}

func (pdi PeerDialInfo) MarshalText() ([]byte, error) {
	return []byte(pdi.TransportName + " " + pdi.DialAddr), nil
}

func (pdi *PeerDialInfo) UnmarshalText(bs []byte) error {
	parts := bytes.SplitN(bs, []byte(" "), 1)
	if len(parts) > 0 {
		pdi.TransportName = string(parts[0])
	}
	if len(parts) > 1 {
		pdi.DialAddr = string(parts[1])
	}
	return nil
}

func (pdi *PeerDialInfo) ScanMapKey(keypath state.Keypath) error {
	parts := bytes.Split(keypath, []byte("|"))
	if len(parts) != 2 {
		return errors.Errorf("bad map keypath for PeerDialInfo: %v", keypath)
	}
	tpt, err := url.QueryUnescape(string(parts[0]))
	if err != nil {
		return err
	}
	dialAddr, err := url.QueryUnescape(string(parts[1]))
	if err != nil {
		return err
	}

	*pdi = PeerDialInfo{tpt, dialAddr}
	return nil
}

func (pdi PeerDialInfo) MapKey() (state.Keypath, error) {
	return state.Keypath(bytes.Join([][]byte{
		[]byte(url.QueryEscape(pdi.TransportName)),
		[]byte(url.QueryEscape(pdi.DialAddr)),
	}, []byte("|"))), nil
}

var (
	ErrNotReady = errors.New("not ready")
)

func NewPeerStore(state *state.DBTree) *peerStore {
	s := &peerStore{
		Logger:                  log.NewLogger("peerstore"),
		state:                   state,
		endpoints:               make(map[PeerDialInfo]*endpoint),
		peersWithAddress:        make(map[types.Address]map[string]struct{}),
		peersWithDeviceUniqueID: make(map[string]*peerDetails),
		unverifiedPeers:         make(map[PeerDialInfo]struct{}),
	}
	s.Infof(0, "opening peer store")

	pds, err := s.fetchAllPeerDetails()
	if err != nil {
		s.Warnf("could not fetch stored peer details from DB: %v", err)
	} else {
		for _, pd := range pds {
			for _, e := range pd.Endpts {
				s.endpoints[e.Dialinfo] = e

				if len(pd.DeviceUniqID) > 0 {
					s.peersWithDeviceUniqueID[pd.DeviceUniqID] = pd
				}

				if len(pd.Addrs) > 0 {
					for addr := range pd.Addrs {
						if _, exists := s.peersWithAddress[addr]; !exists {
							s.peersWithAddress[addr] = make(map[string]struct{})
						}
						s.peersWithAddress[addr][pd.DeviceUniqID] = struct{}{}
					}

				} else {
					s.unverifiedPeers[e.Dialinfo] = struct{}{}
				}
			}
		}
	}

	return s
}

func (s *peerStore) Peers() []PeerInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerInfo
	for _, p := range s.peersWithDeviceUniqueID {
		peers = append(peers, p)
	}
	return peers
}

func (s *peerStore) OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo)) {
	s.newUnverifiedPeerListenersMu.Lock()
	defer s.newUnverifiedPeerListenersMu.Unlock()
	s.newUnverifiedPeerListeners = append(s.newUnverifiedPeerListeners, fn)
}

func (s *peerStore) notifyNewUnverifiedPeerListeners(dialInfo PeerDialInfo) {
	s.newUnverifiedPeerListenersMu.RLock()
	defer s.newUnverifiedPeerListenersMu.RUnlock()
	for _, listener := range s.newUnverifiedPeerListeners {
		listener(dialInfo)
	}
}

func (s *peerStore) OnNewVerifiedPeer(fn func(peer PeerInfo)) {
	s.newVerifiedPeerListenersMu.Lock()
	defer s.newVerifiedPeerListenersMu.Unlock()
	s.newVerifiedPeerListeners = append(s.newVerifiedPeerListeners, fn)
}

func (s *peerStore) notifyNewVerifiedPeerListeners(peer PeerInfo) {
	s.newVerifiedPeerListenersMu.RLock()
	defer s.newVerifiedPeerListenersMu.RUnlock()
	for _, listener := range s.newVerifiedPeerListeners {
		listener(peer)
	}
}

func (s *peerStore) AddDialInfo(dialInfo PeerDialInfo, deviceUniqueID string) PeerEndpoint {
	s.Debugf("AddDialInfo %v (%v)", dialInfo, deviceUniqueID)
	if deviceUniqueID == "" || dialInfo.DialAddr == "" {
		pd := newPeerDetails(s, "")
		e := &endpoint{
			peerDetails: pd,
			Dialinfo:    dialInfo,
		}
		pd.Endpts[dialInfo] = e
		return e
	}

	var endpoint PeerEndpoint
	var exists bool
	func() {
		s.muPeers.Lock()
		defer s.muPeers.Unlock()

		var pd *peerDetails
		var needsSave bool
		pd, exists, needsSave = s.ensurePeerDetails(dialInfo, deviceUniqueID)

		if len(pd.Addrs) == 0 {
			s.unverifiedPeers[dialInfo] = struct{}{}
		}

		if needsSave && deviceUniqueID != "" {
			err := s.savePeerDetails(pd)
			if err != nil {
				s.Warnf("could not save modifications to peerstore DB: %v", err)
			}
		}
		endpoint = pd.Endpts[dialInfo]
	}()
	if !exists && deviceUniqueID != "" {
		s.notifyNewUnverifiedPeerListeners(dialInfo)
	}
	return endpoint
}

func (s *peerStore) findPeerDetails(dialInfo PeerDialInfo, deviceUniqueID string) *peerDetails {
	if deviceUniqueID != "" {
		pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if exists {
			return pd
		}
	}
	e, exists := s.endpoints[dialInfo]
	if !exists {
		return nil
	}
	return e.peerDetails
}

func (s *peerStore) ensurePeerDetails(dialInfo PeerDialInfo, deviceUniqueID string) (pd *peerDetails, knownPeer bool, needsSave bool) {
	pd = s.findPeerDetails(dialInfo, deviceUniqueID)
	knownPeer = pd != nil

	if pd == nil {
		pd = newPeerDetails(s, deviceUniqueID)
		needsSave = true
	}
	if deviceUniqueID != "" {
		s.peersWithDeviceUniqueID[deviceUniqueID] = pd

		// Handle the case where a peer's deviceUniqueID has changed (cleared browser cache, wiped keystore, etc.)
		if pd.DeviceUniqID != deviceUniqueID && pd.DeviceUniqID != "" {
			for addr := range pd.Addrs {
				if _, exists := s.peersWithAddress[addr]; !exists {
					s.peersWithAddress[addr] = make(map[string]struct{})
				}
				s.peersWithAddress[addr][deviceUniqueID] = struct{}{}
			}
			delete(s.peersWithDeviceUniqueID, pd.DeviceUniqID)
			needsSave = true
		}
		pd.DeviceUniqID = deviceUniqueID
	}
	e, known := pd.ensureEndpoint(dialInfo)
	if !known {
		needsSave = true
		s.endpoints[dialInfo] = e
	}
	return
}

func (s *peerStore) AddVerifiedCredentials(
	dialInfo PeerDialInfo,
	deviceUniqueID string,
	address types.Address,
	sigpubkey *crypto.SigningPublicKey,
	encpubkey *crypto.AsymEncPubkey,
) PeerEndpoint {
	if deviceUniqueID == "" || dialInfo.DialAddr == "" {
		pd := newPeerDetails(s, "")
		pd.Addrs = types.NewAddressSet([]types.Address{address})
		pd.Sigpubkeys = map[types.Address]*crypto.SigningPublicKey{address: sigpubkey}
		pd.Encpubkeys = map[types.Address]*crypto.AsymEncPubkey{address: encpubkey}
		e := &endpoint{
			peerDetails: pd,
			Dialinfo:    dialInfo,
		}
		pd.Endpts[dialInfo] = e
		return e
	}

	if address.IsZero() {
		panic("cannot add verified peer without credentials")
		// } else if deviceUniqueID == "" || deviceUniqueID == "0000000000000000000000000000000000000000000000000000000000000000" {
		// 	panic("cannot add verified peer without device unique ID")
	}

	var (
		pd               *peerDetails
		endpoint         PeerEndpoint
		peerAlreadyKnown bool
	)
	func() {
		s.muPeers.Lock()
		defer s.muPeers.Unlock()

		_, peerAlreadyKnown = s.peersWithAddress[address]
		if peerAlreadyKnown {
			_, peerAlreadyKnown = s.peersWithAddress[address][deviceUniqueID]
		}

		pd, _, _ = s.ensurePeerDetails(dialInfo, deviceUniqueID)

		pd.peerStore = s

		pd.Addrs.Add(address)
		if sigpubkey != nil {
			pd.Sigpubkeys[address] = sigpubkey
		}
		if encpubkey != nil {
			pd.Encpubkeys[address] = encpubkey
		}

		if _, exists := s.peersWithAddress[address]; !exists {
			s.peersWithAddress[address] = make(map[string]struct{})
		}
		s.peersWithAddress[address][deviceUniqueID] = struct{}{}

		if dialInfo.DialAddr != "" {
			s.endpoints[dialInfo] = pd.Endpts[dialInfo]
		}

		delete(s.unverifiedPeers, dialInfo)

		err := s.savePeerDetails(pd)
		if err != nil {
			s.Warnf("could not save modifications to peerstore DB: %v", err)
		}
		endpoint = pd.Endpts[dialInfo]
	}()

	if !peerAlreadyKnown {
		s.notifyNewVerifiedPeerListeners(pd)
	}
	return endpoint
}

func (s *peerStore) RemovePeers(deviceUniqueIDs []string) error {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for _, deviceUniqueID := range deviceUniqueIDs {
		pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if !exists {
			continue
		}
		for dialInfo := range pd.Endpts {
			delete(s.endpoints, dialInfo)
			delete(s.unverifiedPeers, dialInfo)
		}
		for addr := range pd.Addrs {
			delete(s.peersWithAddress, addr)
		}
		delete(s.peersWithDeviceUniqueID, pd.DeviceUniqID)
	}
	return s.deletePeers(deviceUniqueIDs)
}

func (s *peerStore) PeerEndpoint(dialInfo PeerDialInfo) PeerEndpoint {
	if dialInfo.DialAddr == "" {
		return nil
	}
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	e, exists := s.endpoints[dialInfo]
	if !exists {
		return nil
	}
	return e
}

func (s *peerStore) UnverifiedPeers() []PeerDialInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	unverifiedPeers := make([]PeerDialInfo, len(s.unverifiedPeers))
	i := 0
	for dialInfo := range s.unverifiedPeers {
		unverifiedPeers[i] = dialInfo
		i++
	}
	return unverifiedPeers
}

func (s *peerStore) VerifiedPeers() []PeerInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var pds []PeerInfo
	for _, peers := range s.peersWithAddress {
		for deviceUniqueID := range peers {
			pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
			if !exists {
				continue
			}
			pds = append(pds, pd)
		}
	}
	return pds
}

func (s *peerStore) AllDialInfos() map[PeerDialInfo]struct{} {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	dialInfos := make(map[PeerDialInfo]struct{})
	for dialInfo := range s.endpoints {
		dialInfos[dialInfo] = struct{}{}
	}
	for dialInfo := range s.unverifiedPeers {
		dialInfos[dialInfo] = struct{}{}
	}
	return dialInfos
}

func (s *peerStore) PeerWithDeviceUniqueID(deviceUniqueID string) (PeerInfo, bool) {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	p, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
	return p, exists
}

func (s *peerStore) PeersWithAddress(address types.Address) []PeerInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerInfo
	for deviceUniqueID := range s.peersWithAddress[address] {
		pd, exists := s.peersWithDeviceUniqueID[deviceUniqueID]
		if !exists {
			continue
		}
		peers = append(peers, pd)
	}
	return peers
}

func (s *peerStore) PeersFromTransport(transportName string) []PeerEndpoint {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var endpoints []PeerEndpoint
	for dialInfo, endpoint := range s.endpoints {
		if dialInfo.TransportName == transportName {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (s *peerStore) PeersServingStateURI(stateURI string) []PeerInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peerInfos []PeerInfo
	for _, p := range s.peersWithDeviceUniqueID {
		if !p.Stateuris.Contains(stateURI) {
			continue
		}
		peerInfos = append(peerInfos, p)
	}
	return peerInfos
}

func (s *peerStore) IsKnownPeer(dialInfo PeerDialInfo) bool {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()
	_, exists := s.endpoints[dialInfo]
	return exists
}

func (s *peerStore) dialInfoHash(dialInfo PeerDialInfo) string {
	return types.HashBytes([]byte(dialInfo.TransportName + ":" + dialInfo.DialAddr)).Hex()
}

func (s *peerStore) fetchAllPeerDetails() (map[string]*peerDetails, error) {
	node := s.state.State(false)
	defer node.Close()

	keypath := state.Keypath("peers")

	var pds map[string]*peerDetails
	err := node.NodeAt(keypath, nil).Scan(&pds)
	if err != nil {
		return nil, err
	}

	for _, pd := range pds {
		pd.peerStore = s
		for _, e := range pd.Endpts {
			e.peerDetails = pd
			e.backoff = utils.ExponentialBackoff{Min: 3 * time.Second, Max: 3 * time.Minute}
		}

		stateURIs := types.NewStringSet(nil)
		for stateURI := range pd.Stateuris {
			x, err := url.QueryUnescape(stateURI)
			if err != nil {
				continue
			}
			stateURIs.Add(x)
		}
		pd.Stateuris = stateURIs
	}
	return pds, nil
}

func (s *peerStore) savePeerDetails(pd *peerDetails) error {
	// node := s.state.State(true)
	// defer node.Close()

	// keypath := state.Keypath("peers").Pushs(pd.DeviceUniqID)

	// stateURIs := types.NewStringSet(nil)
	// for stateURI := range pd.Stateuris {
	// 	stateURIs.Add(url.QueryEscape(stateURI))
	// }
	// old := pd.Stateuris
	// pd.Stateuris = stateURIs
	// defer func() { pd.Stateuris = old }()

	// err := node.Set(keypath, nil, pd)
	// if err != nil {
	// 	return err
	// }
	// return node.Save()
	return nil
}

func (s *peerStore) deletePeers(deviceUniqueIDs []string) error {
	node := s.state.State(true)
	defer node.Close()

	for _, duID := range deviceUniqueIDs {
		peerKeypath := state.Keypath("peers").Pushs(duID)
		err := node.Delete(peerKeypath, nil)
		if err != nil {
			return err
		}
	}
	return node.Save()
}

func (s *peerStore) DebugPrint() {
	node := s.state.State(false)
	defer node.Close()
	node.NodeAt(state.Keypath("peers"), nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
	// s.Debugf("%v", utils.PrettyJSON(node.NodeAt(state.Keypath("peers"), nil)))
}

type PeerInfo interface {
	DeviceUniqueID() string
	SetDeviceUniqueID(id string) error
	Addresses() []types.Address
	PublicKeys(addr types.Address) (*crypto.SigningPublicKey, *crypto.AsymEncPubkey)
	StateURIs() types.StringSet
	AddStateURI(stateURI string) error
	RemoveStateURI(stateURI string) error
	LastContact() time.Time
	LastFailure() time.Time
	Failures() uint64
	Ready() bool
	RemainingBackoff() time.Duration

	Endpoints() map[PeerDialInfo]PeerEndpoint
	Endpoint(dialInfo PeerDialInfo) (PeerEndpoint, bool)
}

type PeerEndpoint interface {
	PeerInfo
	DialInfo() PeerDialInfo
	Dialable() bool
	UpdateConnStats(success bool)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() uint64
	Ready() bool
	RemainingBackoff() time.Duration
}

type peerDetails struct {
	peerStore    *peerStore                                 `tree:"-"`
	DeviceUniqID string                                     `tree:"deviceUniqueID"`
	Addrs        types.AddressSet                           `tree:"addresses"`
	Sigpubkeys   map[types.Address]*crypto.SigningPublicKey `tree:"sigpubkeys"`
	Encpubkeys   map[types.Address]*crypto.AsymEncPubkey    `tree:"encpubkeys"`
	Stateuris    types.StringSet                            `tree:"stateURIs"`
	Endpts       map[PeerDialInfo]*endpoint                 `tree:"endpoints"`
}

func newPeerDetails(peerStore *peerStore, deviceUniqueID string) *peerDetails {
	return &peerDetails{
		peerStore:    peerStore,
		DeviceUniqID: deviceUniqueID,
		Addrs:        types.NewAddressSet(nil),
		Sigpubkeys:   make(map[types.Address]*crypto.SigningPublicKey),
		Encpubkeys:   make(map[types.Address]*crypto.AsymEncPubkey),
		Stateuris:    types.NewStringSet(nil),
		Endpts:       make(map[PeerDialInfo]*endpoint),
	}
}

func (pd *peerDetails) ensureEndpoint(dialInfo PeerDialInfo) (*endpoint, bool) {
	e, exists := pd.Endpts[dialInfo]
	if exists {
		return e, true
	}
	e = &endpoint{
		peerDetails: pd,
		Dialinfo:    dialInfo,
		backoff:     utils.ExponentialBackoff{Min: 3 * time.Second, Max: 3 * time.Minute},
	}
	pd.Endpts[dialInfo] = e
	return e, false
}

func (pd *peerDetails) Addresses() []types.Address {
	pd.rlock()
	defer pd.runlock()
	return pd.Addrs.Slice()
}

func (pd *peerDetails) PublicKeys(addr types.Address) (*crypto.SigningPublicKey, *crypto.AsymEncPubkey) {
	pd.rlock()
	defer pd.runlock()
	return pd.Sigpubkeys[addr], pd.Encpubkeys[addr]
}

func (pd *peerDetails) DeviceUniqueID() string {
	pd.rlock()
	defer pd.runlock()
	return pd.DeviceUniqID
}

func (pd *peerDetails) SetDeviceUniqueID(id string) error {
	if id == "" {
		return nil
	}
	pd.lock()
	defer pd.unlock()
	pd.DeviceUniqID = id
	return pd.peerStore.savePeerDetails(pd)
}

func (pd *peerDetails) StateURIs() types.StringSet {
	pd.rlock()
	defer pd.runlock()
	return pd.Stateuris.Copy()
}

func (pd *peerDetails) AddStateURI(stateURI string) error {
	pd.lock()
	defer pd.unlock()
	pd.Stateuris.Add(stateURI)
	return pd.peerStore.savePeerDetails(pd)
}

func (pd *peerDetails) RemoveStateURI(stateURI string) error {
	pd.lock()
	defer pd.unlock()
	pd.Stateuris.Remove(stateURI)
	return pd.peerStore.savePeerDetails(pd)
}

func (pd *peerDetails) LastContact() time.Time {
	pd.rlock()
	defer pd.runlock()
	var lastContact time.Time
	for _, e := range pd.Endpts {
		if time.Time(e.Lastcontact).After(lastContact) {
			lastContact = time.Time(e.Lastcontact)
		}
	}
	return lastContact
}

func (pd *peerDetails) LastFailure() time.Time {
	pd.rlock()
	defer pd.runlock()
	var lastFailure time.Time
	for _, e := range pd.Endpts {
		if time.Time(e.Lastfailure).After(lastFailure) {
			lastFailure = time.Time(e.Lastfailure)
		}
	}
	return lastFailure
}

func (pd *peerDetails) Failures() uint64 {
	pd.rlock()
	defer pd.runlock()
	var failures uint64
	for _, e := range pd.Endpts {
		failures += e.Fails
	}
	return failures
}

func (pd *peerDetails) Ready() bool {
	pd.rlock()
	defer pd.runlock()
	for _, e := range pd.Endpts {
		ready, _ := e.backoff.Ready()
		if ready {
			return true
		}
	}
	return false
}

func (pd *peerDetails) RemainingBackoff() time.Duration {
	pd.rlock()
	defer pd.runlock()
	minRemaining := time.Duration(math.MaxInt64)
	for _, e := range pd.Endpts {
		_, remaining := e.backoff.Ready()
		if remaining < minRemaining {
			minRemaining = remaining
		}
	}
	return minRemaining
}

func (pd *peerDetails) Endpoints() map[PeerDialInfo]PeerEndpoint {
	pd.rlock()
	defer pd.runlock()
	endpoints := make(map[PeerDialInfo]PeerEndpoint, len(pd.Endpts))
	for dialInfo, e := range pd.Endpts {
		endpoints[dialInfo] = e
	}
	return endpoints
}

func (pd *peerDetails) Endpoint(dialInfo PeerDialInfo) (PeerEndpoint, bool) {
	pd.rlock()
	defer pd.runlock()
	e, exists := pd.Endpts[dialInfo]
	return e, exists
}

func (pd *peerDetails) lock()    { pd.peerStore.muPeers.Lock() }
func (pd *peerDetails) unlock()  { pd.peerStore.muPeers.Unlock() }
func (pd *peerDetails) rlock()   { pd.peerStore.muPeers.RLock() }
func (pd *peerDetails) runlock() { pd.peerStore.muPeers.RUnlock() }

type endpoint struct {
	*peerDetails `tree:"-"`
	Dialinfo     PeerDialInfo             `tree:"dialInfo"`
	Lastcontact  types.Time               `tree:"lastContact"`
	Lastfailure  types.Time               `tree:"lastFailure"`
	Fails        uint64                   `tree:"failures"`
	backoff      utils.ExponentialBackoff `tree:"-"`
}

func (e *endpoint) Dialable() bool {
	e.rlock()
	defer e.runlock()
	return e.Dialinfo.DialAddr != ""
}

func (e *endpoint) DialInfo() PeerDialInfo {
	e.rlock()
	defer e.runlock()
	return e.Dialinfo
}

func (e *endpoint) UpdateConnStats(success bool) {
	e.lock()
	defer e.unlock()

	now := types.Time(time.Now())
	if success {
		e.Lastcontact = now
		e.Fails = 0
	} else {
		e.Lastfailure = now
		e.Fails++
		e.backoff.Next()
	}
	e.peerDetails.peerStore.savePeerDetails(e.peerDetails)
}

func (e *endpoint) LastContact() time.Time {
	e.rlock()
	defer e.runlock()
	return time.Time(e.Lastcontact)
}

func (e *endpoint) LastFailure() time.Time {
	e.rlock()
	defer e.runlock()
	return time.Time(e.Lastfailure)
}

func (e *endpoint) Failures() uint64 {
	e.rlock()
	defer e.runlock()
	return e.Fails
}

// @@TODO: this should be configurable and probably not handled here
func (e *endpoint) Ready() bool {
	e.rlock()
	defer e.runlock()
	ready, _ := e.backoff.Ready()
	return ready
}

func (e *endpoint) RemainingBackoff() time.Duration {
	e.rlock()
	defer e.runlock()
	_, remaining := e.backoff.Ready()
	return remaining
}
