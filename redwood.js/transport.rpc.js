

async function rpcFetch(method, params) {
    let resp = await (await fetch('http://localhost:8081', {
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

export default {
    rpcFetch,

    subscribe: function ({ stateURI, txs, states, keypath }) {
        return rpcFetch('RPC.Subscribe', { stateURI, txs, states, keypath })
    },

    nodeAddress: async function ({ stateURI, txs, states, keypath }) {
        return (await rpcFetch('RPC.NodeAddress')).Address
    },

    knownStateURIs: async function ({ stateURI, txs, states, keypath }) {
        return (await rpcFetch('RPC.KnownStateURIs')).StateURIs
    },

    sendTx: function (tx) {
        return rpcFetch('RPC.SendTx', { tx })
    },
}
