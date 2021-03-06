import { RPCClient, Tx, RPCSubscribeParams, RPCIdentitiesResponse } from './types'

async function rpcFetch(endpoint: string, method: string, params?: {[key: string]: any}) {
    let resp = await (await fetch(endpoint, {
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

export default function createRPCClient({ endpoint }: { endpoint?: string }): RPCClient {
    let _endpoint = endpoint || 'http://localhost:8081'
    return {
        rpcFetch: (method: string, params?: {[key: string]: any}) => rpcFetch(_endpoint, method, params),

        subscribe: async function ({ stateURI, keypath, txs, states }: RPCSubscribeParams) {
            await rpcFetch(_endpoint, 'RPC.Subscribe', { stateURI, txs, states, keypath })
        },

        identities: async function (): Promise<RPCIdentitiesResponse[]> {
            return ((await rpcFetch(_endpoint, 'RPC.Identities')).Identities as { Address: string, Public: boolean }[])
                          .map(({ Address, Public }) => ({ address: Address, public: Public }))
        },

        newIdentity: async function () {
            return (await rpcFetch(_endpoint, 'RPC.NewIdentity')).Address as string
        },

        knownStateURIs: async function () {
            return (await rpcFetch(_endpoint, 'RPC.KnownStateURIs')).StateURIs as string[]
        },

        sendTx: async function (tx: Tx) {
            await rpcFetch(_endpoint, 'RPC.SendTx', { Tx: tx })
        },

        addPeer: async function ({ transportName, dialAddr }: { transportName: string, dialAddr: string }) {
            await rpcFetch(_endpoint, 'RPC.AddPeer', { TransportName: transportName, DialAddr: dialAddr })
        },
    }
}
