require('@babel/polyfill')

var identity = require('./identity')
var sync9 = require('./sync9-src')
var utils = require('./utils')

window.Braid = {
    get,
    subscribe,
    put,
    storeRef,

    // submodules
    identity,
    sync9,
    utils,
}


async function subscribe(host, keypath, parents, cb) {
    try {
        const headers = {
            'Subscribe-Host': host,
            'Accept':         'application/json',
            'Subscribe':      'keep-alive',
        }
        if (parents && parents.length > 0) {
            headers['Parents'] = parents.join(',')
        }

        const res = await fetch(keypath, {
            method: 'GET',
            headers,
        })
        if (!res.ok) {
            console.error('Fetch failed!', res)
            cb('fetch failed')
            return
        }
        const reader = res.body.getReader()
        const decoder = new TextDecoder('utf-8')
        let buffer = ''

        async function read() {
            const x = await reader.read()
            if (x.done) {
                return
            }

            const newData = decoder.decode(x.value)
            buffer += newData
            let idx
            while ((idx = buffer.indexOf('\n')) > -1) {
                const line = buffer.substring(0, idx).trim()
                if (line.length > 0) {
                    const payloadStr = line.substring(5).trim() // remove "data:" prefix
                    let payload
                    try {
                        payload = JSON.parse(payloadStr)
                    } catch (err) {
                        console.error('Error parsing JSON:', payloadStr)
                        throw err
                    }
                    cb(null, payload)
                }
                buffer = buffer.substring(idx+1)
            }
            read()
        }
        read()

    } catch(err) {
        console.log('Fetch GET failed:', err)
        cb(err)
    }
}

async function get(keypath) {
    return (await (await fetch(keypath, {
        headers: {
            'Accept': 'application/json',
        },
    })).json())
}

function put(tx, identity) {
    tx.from = identity.address
    var sig = identity.signTx(tx)

    const headers = {
        'Version': tx.id,
        'Parents': tx.parents.join(','),
        'Signature': sig,
        'Patch-Type': 'braid',
        'State-URL': tx.url, // @@TODO: bad
    }

    return fetch('/', {
        method: 'PUT',
        body: tx.patches.join('\n'),
        headers,
    })
}

async function storeRef(file) {
    const formData = new FormData()
    formData.append(`ref`, file)

    const resp = await fetch(`/`, {
        method: 'PUT',
        headers: { 'Ref': 'true' },
        body: formData,
    })

    return (await resp.json()).hash
}



