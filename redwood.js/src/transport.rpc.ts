import { RPCClient, RPCSendTx, RPCSubscribeParams, RPCIdentitiesResponse, RPCPeer, RPCPeerIdentity } from './types'

let theFetch: typeof fetch = typeof window !== 'undefined'
                                ? fetch
                                : require('node-fetch')

async function rpcFetch(endpoint: string, method: string, params?: {[key: string]: any}) {
    let resp = await (await theFetch(endpoint, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            jsonrpc: '2.0',
            method,
            params,
            id: 0,
        }),
    })).json()

    if (resp.error) {
        throw new Error(resp.error.message)
    }
    return resp.result
}

export default function createRPCClient({ endpoint }: { endpoint: string }): RPCClient {
    if (!endpoint) {
        throw new Error('RPC client requires an endpoint')
    }
    return {
        rpcFetch: (method: string, params?: {[key: string]: any}) => rpcFetch(endpoint, method, params),

        ucan: async function() {
            return (await rpcFetch(endpoint, 'RPC.Ucan')).JWT
        },

        subscribe: async function ({ stateURI, keypath, txs, states }: RPCSubscribeParams) {
            await rpcFetch(endpoint, 'RPC.Subscribe', { stateURI, txs, states, keypath })
        },

        identities: async function (): Promise<RPCIdentitiesResponse[]> {
            return ((await rpcFetch(endpoint, 'RPC.Identities')).Identities as { Address: string, Public: boolean }[])
                          .map(({ Address, Public }) => ({ address: Address, public: Public }))
        },

        newIdentity: async function () {
            return (await rpcFetch(endpoint, 'RPC.NewIdentity')).Address as string
        },

        knownStateURIs: async function () {
            return (await rpcFetch(endpoint, 'RPC.KnownStateURIs')).StateURIs as string[]
        },

        sendTx: async function (tx: RPCSendTx) {
            await rpcFetch(endpoint, 'RPC.SendTx', { Tx: tx })
        },

        addPeer: async function ({ transportName, dialAddr }: { transportName: string, dialAddr: string }) {
            await rpcFetch(endpoint, 'RPC.AddPeer', { TransportName: transportName, DialAddr: dialAddr })
        },

        staticRelays: async function (): Promise<string[]> {
            return (await rpcFetch(endpoint, 'RPC.StaticRelays', {})).StaticRelays || []
        },

        addStaticRelay: async function (dialAddr: string) {
            await rpcFetch(endpoint, 'RPC.AddStaticRelay', { DialAddr: dialAddr })
        },

        removeStaticRelay: async function (dialAddr: string) {
            await rpcFetch(endpoint, 'RPC.RemoveStaticRelay', { DialAddr: dialAddr })
        },

        privateTreeMembers: async function (stateURI: string) {
            try {
                return (await rpcFetch(endpoint, 'RPC.PrivateTreeMembers', { StateURI: stateURI })).Members as string[]
            } catch (err) {
                if (`${err}`.indexOf('no controller for that stateURI') > -1) {
                    return []
                }
                throw err
            }
        },

        peers: async function (): Promise<RPCPeer[]> {
            let peers = (await rpcFetch(endpoint, 'RPC.Peers')).Peers as {
                Identities:          { Address: string, SigningPublicKey: string, EncryptingPublicKey: string }[]
                Transport:           string
                DialAddr:            string
                StateURIs:           string[]
                LastContact:         number
            }[]
            return peers.map(peer => ({
                identities:  (peer.Identities || []).map(i => ({
                    address:             i.Address,
                    signingPublicKey:    i.SigningPublicKey,
                    encryptingPublicKey: i.EncryptingPublicKey,
                })),
                transport:   peer.Transport,
                dialAddr:    peer.DialAddr,
                stateURIs:   peer.StateURIs,
                lastContact: peer.LastContact > 0 ? new Date(peer.LastContact * 1000) : null,
            }))
        },
    }
}
