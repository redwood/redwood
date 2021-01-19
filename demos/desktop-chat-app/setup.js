const fs = require('fs')
const Braid = require('./frontend/src/braidjs/braid-src.js')

let braidIdentity = Braid.identity.random()
let braidClient = Braid.createPeer({
    identity: braidIdentity,
    httpHost: 'http://localhost:8080',
    onFoundPeersCallback: (peers) => {}
})

async function main() {
    await braidClient.authorize()

    let sync9JS = fs.createReadStream('./frontend/src/braidjs/dist/sync9-otto.js')
    let { sha3: sync9JSSha3 } = await braidClient.storeRef(sync9JS)
    console.log('sync9 sha:', sync9JSSha3)

    let tx = {
        stateURI: 'chat.redwood.dev/registry',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^.*$': {
                                'write': true,
                            },
                        },
                    },
                },
                'rooms': [],
            }),
        ],
    }
    await rpcFetch('RPC.Subscribe', { StateURI: 'chat.redwood.dev/registry' })
    await rpcFetch('RPC.SendTx', { Tx: tx })

    console.log('Done.')
    process.exit(0)
}

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

main()
