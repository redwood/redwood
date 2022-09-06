package vault

import (
	"context"
	"time"

	"go.uber.org/multierr"

	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
)

type ClientManager struct {
	process.Process
	log.Logger
	store         ClientStore
	keyStore      identity.KeyStore
	clients       SyncMap[string, *Client]
	syncWithVault VaultClientFunc

	syncWithVaultsTask *syncWithVaultsTask
}

type VaultClientFunc func(ctx context.Context, client *Client) error

type ClientStore interface {
	Vaults() Set[string]
	AddVault(host string) error
	RemoveVault(host string) error
	LatestMtimeForVaultAndCollection(host, collectionID string) time.Time
}

func NewClientManager(
	name string,
	store ClientStore,
	keyStore identity.KeyStore,
	syncInterval time.Duration,
	syncWithVault VaultClientFunc,
) *ClientManager {
	m := &ClientManager{
		Process:       *process.New("vault manager"),
		Logger:        log.NewLogger(name),
		store:         store,
		keyStore:      keyStore,
		clients:       NewSyncMap[string, *Client](),
		syncWithVault: syncWithVault,
	}
	m.syncWithVaultsTask = newSyncWithVaultsTask(syncInterval, m)
	return m
}

func (m *ClientManager) Start() error {
	err := m.Process.Start()
	if err != nil {
		return err
	}

	err = m.Process.SpawnChild(nil, m.syncWithVaultsTask)
	if err != nil {
		return err
	}

	m.syncWithVaultsTask.Enqueue()
	return nil
}

func (m *ClientManager) Close() error {
	m.clients.Range(func(host string, client *Client) bool {
		client.Close()
		return true
	})
	return m.Process.Close()
}

func (m *ClientManager) Vaults() Set[string] {
	return m.store.Vaults()
}

func (m *ClientManager) AddVault(host string) error {
	err := m.store.AddVault(host)
	if err != nil {
		return err
	}

	_, exists := m.clients.Get(host)
	if !exists {
		ctx, cancel := context.WithTimeout(m.Process.Ctx(), 30*time.Second)

		m.Process.Go(ctx, "sync with vault "+host, func(ctx context.Context) {
			defer cancel()

			err := m.WithVaultClient(ctx, host, func(ctx context.Context, client *Client) error {
				return m.syncWithVault(ctx, client)
			})
			if err != nil {
				m.Errorw("failed to sync with vault", "host", host, "err", err)
			}
		})
	}
	return nil
}

func (m *ClientManager) RemoveVault(host string) error {
	client, exists := m.clients.Delete(host)
	if exists {
		client.Close()
	}
	return m.store.RemoveVault(host)
}

func (m *ClientManager) SyncWithAllVaults(ctx context.Context) error {
	return m.WithAllVaultClients(ctx, "sync", m.syncWithVault)
}

func (m *ClientManager) WithAllVaultClients(ctx context.Context, processName string, fn VaultClientFunc) error {
	child := m.Process.NewChild(ctx, processName)
	defer child.Close()

	n := m.store.Vaults().Length()
	chErr := make(chan error, n)
	for _, host := range m.store.Vaults().Slice() {
		host := host
		child.Go(ctx, host, func(ctx context.Context) {
			chErr <- m.WithVaultClient(ctx, host, fn)
		})
	}
	return multierr.Combine(utils.CollectChan(ctx, n, chErr)...)
}

func (m *ClientManager) WithVaultClient(ctx context.Context, host string, fn VaultClientFunc) error {
	identity, err := m.keyStore.DefaultPublicIdentity()
	if err != nil {
		m.Errorf("while fetching default public identity from key store: %v", err)
		return err
	}

	client, exists := m.clients.Get(host)
	if !exists {
		client = NewClient(host, identity, m.store)
		err = client.Dial(ctx)
		if err != nil {
			m.Errorw("failed to dial vault", "host", host, "err", err)
			return err
		}
		m.clients.Set(host, client)
	}
	return fn(ctx, client)
}

type syncWithVaultsTask struct {
	process.PeriodicTask
	vaultManager *ClientManager
}

func newSyncWithVaultsTask(
	interval time.Duration,
	vaultManager *ClientManager,
) *syncWithVaultsTask {
	t := &syncWithVaultsTask{
		vaultManager: vaultManager,
	}
	t.PeriodicTask = *process.NewPeriodicTask("SyncWithVaultsTask", utils.NewStaticTicker(interval), func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, interval)
		defer cancel()

		err := t.vaultManager.SyncWithAllVaults(ctx)
		if err != nil {
			t.vaultManager.Errorf("while syncing with all vaults: %v", err)
		}
	})
	return t
}
