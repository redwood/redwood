import {
    RPCClient,
    Tx,
    RPCSubscribeParams,
    RPCIdentitiesResponse,
    RPCPeer,
    RPCPeerIdentity,
} from './types'

const theFetch: typeof fetch =
    typeof window !== 'undefined' ? fetch : require('node-fetch')

async function rpcFetch(
    endpoint: string,
    method: string,
    params?: { [key: string]: any },
) {
    const resp = await (
        await theFetch(endpoint, {
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
        })
    ).json()

    if (resp.error) {
        throw new Error(resp.error.message)
    }
    return resp.result
}

export default function createRPCClient({
    endpoint,
}: {
    endpoint: string
}): RPCClient {
    if (!endpoint) {
        throw new Error('RPC client requires an endpoint')
    }
    return {
        rpcFetch: (method: string, params?: { [key: string]: any }) =>
            rpcFetch(endpoint, method, params),

        async subscribe({
            stateURI,
            keypath,
            txs,
            states,
        }: RPCSubscribeParams) {
            await rpcFetch(endpoint, 'RPC.Subscribe', {
                stateURI,
                txs,
                states,
                keypath,
            })
        },

        async identities(): Promise<RPCIdentitiesResponse[]> {
            return (
                (await rpcFetch(endpoint, 'RPC.Identities')).Identities as {
                    Address: string
                    Public: boolean
                }[]
            ).map(({ Address, Public }) => ({
                address: Address,
                public: Public,
            }))
        },

        async newIdentity() {
            return (await rpcFetch(endpoint, 'RPC.NewIdentity'))
                .Address as string
        },

        async knownStateURIs() {
            return (await rpcFetch(endpoint, 'RPC.KnownStateURIs'))
                .StateURIs as string[]
        },

        async sendTx(tx: Tx) {
            await rpcFetch(endpoint, 'RPC.SendTx', { Tx: tx })
        },

        async addPeer({
            transportName,
            dialAddr,
        }: {
            transportName: string
            dialAddr: string
        }) {
            await rpcFetch(endpoint, 'RPC.AddPeer', {
                TransportName: transportName,
                DialAddr: dialAddr,
            })
        },

        async privateTreeMembers(stateURI: string) {
            try {
                return (
                    await rpcFetch(endpoint, 'RPC.PrivateTreeMembers', {
                        StateURI: stateURI,
                    })
                ).Members as string[]
            } catch (err: any) {
                if (
                    err.toString().indexOf('no controller for that stateURI') >
                    -1
                ) {
                    return []
                }
                throw err
            }
        },

        async peers(): Promise<RPCPeer[]> {
            const peers = (await rpcFetch(endpoint, 'RPC.Peers')).Peers as {
                Identities: {
                    Address: string
                    SigningPublicKey: string
                    EncryptingPublicKey: string
                }[]
                Transport: string
                DialAddr: string
                StateURIs: string[]
                LastContact: number
            }[]
            return peers.map((peer) => ({
                identities: (peer.Identities || []).map((i) => ({
                    address: i.Address,
                    signingPublicKey: i.SigningPublicKey,
                    encryptingPublicKey: i.EncryptingPublicKey,
                })),
                transport: peer.Transport,
                dialAddr: peer.DialAddr,
                stateURIs: peer.StateURIs,
                lastContact:
                    peer.LastContact > 0
                        ? new Date(peer.LastContact * 1000)
                        : null,
            }))
        },
    }
}
