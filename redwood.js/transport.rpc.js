

async function rpcFetch(endpoint, method, params) {
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

export default function createRPCClient({ endpoint }) {
    endpoint = endpoint || 'http://localhost:8081'
    return {
        rpcFetch: (...args) => rpcFetch(endpoint, ...args),

        subscribe: function ({ stateURI, txs, states, keypath }) {
            return rpcFetch(endpoint, 'RPC.Subscribe', { stateURI, txs, states, keypath })
        },

        nodeAddress: async function ({ stateURI, txs, states, keypath }) {
            return (await rpcFetch(endpoint, 'RPC.NodeAddress')).Address
        },

        knownStateURIs: async function ({ stateURI, txs, states, keypath }) {
            return (await rpcFetch(endpoint, 'RPC.KnownStateURIs')).StateURIs
        },

        sendTx: function (tx) {
            return rpcFetch(endpoint, 'RPC.SendTx', { Tx: tx })
        },

        addPeer: function ({ transportName, dialAddr }) {
            return rpcFetch(endpoint, 'RPC.AddPeer', { TransportName: transportName, DialAddr: dialAddr })
        },
    }
}
