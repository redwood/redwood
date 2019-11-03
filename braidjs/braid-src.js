require('@babel/polyfill')

var ethers = require('ethers')

window.Braid = {
    identityFromMnemonic,
    identityFromPrivateKey,
    randomIdentity,

    subscribePatches,
    subscribeStates,
    put,
    storeRef,

    util: {
        hashTx,
        serializeTx,
        randomID,
        stringToHex,
        randomString,
        hexToUint8Array,
        uint8ArrayToHex,
    },
}

function identityFromMnemonic(mnemonic) {
    return _constructIdentity(ethers.Wallet.fromMnemonic(mnemonic))
}

function identityFromPrivateKey(privateKey) {
    if (privateKey.indexOf('0x') !== 0) {
        privateKey = '0x' + privateKey
    }
    return _constructIdentity(new ethers.Wallet(privateKey))
}

function randomIdentity() {
    return _constructIdentity(ethers.Wallet.createRandom())
}

function _constructIdentity(wallet) {
    var address = wallet.address.slice(2)

    return {
        wallet: wallet,
        address: address,
        signTx: function(tx) {
            var txHash = hashTx(tx)
            var signed = wallet.signingKey.signDigest(txHash)
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam
        },
    }
}

async function subscribePatches(host, keypath, parents, cb) {
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
                    const payload = JSON.parse(payloadStr)
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

function subscribeStates(host, keypath, parents, cb) {
    setInterval(async () => {
        try {
            var resp = (await (await fetch(keypath, {
                headers: {
                    'Accept': 'application/json',
                },
            })).json())
            cb(null, resp)
        } catch (err) {
            cb(err)
        }
    }, 1000)
}

function put(tx, identity) {
    tx.from = identity.address
    var sig = identity.signTx(tx)

    const headers = {
        'Version': tx.id,
        'Parents': tx.parents.join(','),
        'Signature': sig,
        'Patch-Type': 'braid',
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

function hashTx(tx) {
    var txHex = serializeTx(tx)
    return ethers.utils.keccak256(Buffer.from(txHex, 'hex')).toString('hex')
}

function serializeTx(tx) {
    var txHex = ''
    txHex += tx.id
    tx.parents.forEach(parent => txHex += parent)
    txHex += stringToHex(tx.url)
    tx.patches.forEach(patch => txHex += stringToHex(patch))
    return txHex
}

function randomID() {
    return stringToHex(randomString(32))
}

function stringToHex(s) {
    return Buffer.from(s, 'utf8').toString('hex')
}

function randomString(length) {
    var result           = ''
    var characters       = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
    var charactersLength = characters.length
    for (var i = 0; i < length; i++) {
        result += characters.charAt(Math.floor(Math.random() * charactersLength))
    }
    return result
}

function hexToUint8Array(hexString) {
    return new Uint8Array(hexString.match(/.{1,2}/g).map(byte => parseInt(byte, 16)))
}

function uint8ArrayToHex(bytes) {
    return bytes.reduce((str, byte) => str + byte.toString(16).padStart(2, '0'), '')
}
