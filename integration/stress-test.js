const Redwood = require('@redwood.dev/client').default

const rpcURL = 'https://127.0.0.1:8081'
const httpsURL = 'https://127.0.0.1:8082'

let txsSent = 0
let x = {}

async function main() {
    const client = Redwood.createPeer({
        httpHost: 'https://127.0.0.1:8082',
        identity: Redwood.identity.random(),
        rpcEndpoint: 'https://127.0.0.1:8081',
    })
    await client.rpc.subscribe({
        stateURI: 'foo.bar/baz',
        keypath: '/',
        // txs: true,
        states: true,
    })

    client.rpc.sendTx({
        id: Redwood.utils.genesisTxID,
        stateURI: 'foo.bar/baz',
        patches: [
            `. = ${Redwood.utils.JSON.stringify({
            'Merge-Type': {
                'Content-Type': 'resolver/dumb',
                'value':        {},
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
            'users': {},
        })}`
        ],
    })

    for (let i = 0; i < 100; i++) {
        subscribe(i)
    }

    for (let i = 0; i < 100; i++) {
        makeTxs()
    }
}

function subscribe(i) {
    const client = Redwood.createPeer({
        httpHost: 'https://127.0.0.1:8082',
        identity: Redwood.identity.random(),
        rpcEndpoint: 'https://127.0.0.1:8081',
    })
    client.subscribe({
        stateURI: 'foo.bar/baz',
        callback: (err, update) => {
            x[i] = x[i] || 0
            x[i]++
        },
        useWebsocket: true,
        states: true,
        fromTxID: Redwood.utils.genesisTxID,
    })
}

async function makeTxs() {
    const client = Redwood.createPeer({
        httpHost: 'https://127.0.0.1:8082',
        identity: Redwood.identity.random(),
        rpcEndpoint: 'https://127.0.0.1:8081',
    })

    while (true) {
        client.rpc.sendTx({
            id: Redwood.utils.randomID(),
            stateURI: 'foo.bar/baz',
            patches: [
                `.users.${Redwood.utils.randomID()} = {"foo": "bar"}`,
            ],
        })
        txsSent++
        await sleep(1000)
    }
}

function sleep(ms) {
    return new Promise(resolve => {
        setTimeout(resolve, ms)
    })
}

function jitter(ms) {
    return 500 + Math.floor(Math.random() * ms)
}


main()