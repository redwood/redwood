const fs = require('fs')
const FormData = require('form-data')
const stableStringify = require('json-stable-stringify')
require('isomorphic-fetch')

async function main() {
    let sync9JS = fs.createReadStream('./sync9.js')
    let { sha3 } = await storeRef(sync9JS)
    console.log('sync9 sha3:', sha3)

    // let tx = {
    //     stateURI: 'chat.redwood.dev/registry',
    //     id: '67656e6573697300000000000000000000000000000000000000000000000000',
    //     parents: [],
    //     patches: [
    //         ' = ' + stableStringify({
    //             'Merge-Type': {
    //                 'Content-Type': 'resolver/dumb',
    //                 'value': {}
    //             },
    //             'Validator': {
    //                 'Content-Type': 'validator/permissions',
    //                 'value': {
    //                     '*': {
    //                         '^.*$': {
    //                             'write': true,
    //                         },
    //                     },
    //                 },
    //             },
    //             'rooms': [],
    //         }),
    //     ],
    // }
    // await rpcFetch('RPC.Subscribe', { StateURI: 'chat.redwood.dev/registry' })
    // await rpcFetch('RPC.SendTx', { Tx: tx })

    console.log('Done.')
    process.exit(0)
}

async function storeRef(file) {
    let formData
    formData = new FormData()
    formData.append('ref', file)

    const resp = await fetch('http://localhost:8080', {
        method:  'PUT',
        headers: { 'Ref': 'true' },
        body:    formData,
    })

    return (await resp.json())
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
