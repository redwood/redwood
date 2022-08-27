package protohush

import (
	"bytes"
	"context"
	"io/ioutil"
	"time"

	"redwood.dev/identity"
	"redwood.dev/types"
	"redwood.dev/utils"
	"redwood.dev/vault"
)

type VaultManager struct {
	*vault.ClientManager
	store         Store
	pubkeyBundles *utils.Mailbox[vaultItem[PubkeyBundle]]
}

type vaultItem[T any] struct {
	vault.ClientItem
	Item T
}

func NewVaultManager(store Store, keyStore identity.KeyStore, syncInterval time.Duration) *VaultManager {
	vm := &VaultManager{
		store:         store,
		pubkeyBundles: utils.NewMailbox[vaultItem[PubkeyBundle]](1000),
	}
	vm.ClientManager = vault.NewClientManager(ProtocolName, store, keyStore, syncInterval, vm.syncWithVault)
	return vm
}

func (vm *VaultManager) PubkeyBundles() *utils.Mailbox[vaultItem[PubkeyBundle]] {
	return vm.pubkeyBundles
}

func (vm *VaultManager) syncWithVault(ctx context.Context, client *vault.Client) error {
	// Fetch remote pubkey bundles
	ch, err := client.FetchLatestFromCollection(ctx, collectionIDForPubkeyBundles())
	if err != nil {
		return err
	}
	vm.deliverPubkeyBundles(ctx, ch, client.Host())

	// Store our own pubkey bundles
	keyBundles, err := vm.store.KeyBundles()
	if err != nil {
		vm.Errorf("failed to fetch pubkey bundles from store", "err", err)
		return err
	}

	var bundles []PubkeyBundle
	for _, b := range keyBundles {
		bundle, err := vm.store.PubkeyBundle(b.ID())
		if err != nil {
			vm.Errorw("failed to fetch pubkey bundle", "id", b.ID(), "err", err)
			continue
		}
		bundles = append(bundles, bundle)
	}

	reader := bytes.NewReader(nil)

	for _, pubkey := range bundles {
		addr, err := pubkey.Address()
		if err != nil {
			vm.Warnf("while extracting address from pubkey bundle: %v", err)
			continue
		}

		bs, err := pubkey.Marshal()
		if err != nil {
			vm.Warnf("while marshaling pubkey bundle: %v", err)
			continue
		}

		reader.Reset(bs)
		err = client.Store(ctx, collectionIDForUserPubkeyBundles(addr), pubkey.ID().Hex(), reader)
		if err != nil {
			vm.Warnf("while storing pubkey bundle in vault: %v", err)
			continue
		}

		reader.Reset(bs)
		err = client.Store(ctx, collectionIDForPubkeyBundles(), pubkey.ID().Hex(), reader)
		if err != nil {
			vm.Warnf("while storing pubkey bundle in vault: %v", err)
			continue
		}
	}
	return nil
}

func (vm *VaultManager) deliverPubkeyBundles(ctx context.Context, ch <-chan vault.ClientItem, host string) {
	for {
		select {
		case <-ctx.Done():
			return

		case item, open := <-ch:
			if !open {
				return
			}

			func() {
				defer item.Close()

				bs, err := ioutil.ReadAll(item)
				if err != nil {
					vm.Errorw("failed to read pubkey bundle from vault", "host", host, "err", err)
					return
				}

				var bundle PubkeyBundle
				err = bundle.Unmarshal(bs)
				if err != nil {
					vm.Errorw("failed to unmarshal pubkey bundle from vault", "host", host, "err", err)
					return
				}

				vm.pubkeyBundles.Deliver(vaultItem[PubkeyBundle]{item, bundle})
			}()
		}
	}
}

func (vm *VaultManager) FetchPubkeyBundleByID(ctx context.Context, id types.Hash) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	vm.WithAllVaultClients(ctx, "fetch pubkey bundle "+id.Hex(), func(ctx context.Context, client *vault.Client) error {
		item, err := client.Fetch(ctx, collectionIDForPubkeyBundles(), id.Hex())
		if err != nil {
			vm.Errorf("while fetching pubkey bundle (vault=%v id=%v): %v", client.Host(), id, err)
			return err
		}

		bs, err := ioutil.ReadAll(item)
		if err != nil {
			vm.Errorf("while reading pubkey bundle bytes (vault=%v id=%v): %v", client.Host(), id, err)
			return err
		}

		var pubkey PubkeyBundle
		err = pubkey.Unmarshal(bs)
		if err != nil {
			vm.Errorw("failed to unmarshal pubkey bundle", "host", client.Host(), "bundle", id, "err", err)
			return err
		}

		cancel()
		vm.pubkeyBundles.Deliver(vaultItem[PubkeyBundle]{item, pubkey})
		return nil
	})
}

func (vm *VaultManager) FetchPubkeyBundlesForAddress(ctx context.Context, addr types.Address) {
	vm.WithAllVaultClients(ctx, "fetch pubkey bundles for "+addr.Hex(), func(ctx context.Context, client *vault.Client) error {
		itemIDs, err := client.Items(ctx, collectionIDForUserPubkeyBundles(addr), time.Time{})
		if err != nil {
			return err
		}

		var bundles []vaultItem[PubkeyBundle]
		for _, itemID := range itemIDs {
			item, err := client.Fetch(ctx, collectionIDForUserPubkeyBundles(addr), itemID)
			if err != nil {
				vm.Errorf("while fetching pubkey bundle (bundle=%v collection=%v host=%v): %v", itemID, collectionIDForUserPubkeyBundles(addr), client.Host())
				continue
			}

			bs, err := ioutil.ReadAll(item)
			if err != nil {
				vm.Errorf("while reading pubkey bundle (bundle=%v collection=%v host=%v): %v", itemID, collectionIDForUserPubkeyBundles(addr), client.Host())
				continue
			}

			var bundle PubkeyBundle
			err = bundle.Unmarshal(bs)
			if err != nil {
				vm.Errorf("while unmarshaling pubkey bundle (bundle=%v collection=%v host=%v): %v", itemID, collectionIDForUserPubkeyBundles(addr), client.Host())
				continue
			}
			bundles = append(bundles, vaultItem[PubkeyBundle]{item, bundle})
		}

		vm.Debugf("fetched %v pubkey bundles for address %v", len(bundles), addr)

		vm.pubkeyBundles.DeliverAll(bundles)
		return nil
	})
}

func collectionIDForPubkeyBundles() string {
	return "pubkey-bundles"
}

func collectionIDForUserPubkeyBundles(userAddr types.Address) string {
	return "pubkey-bundles-" + types.HashBytes(userAddr[:]).Hex()
}
