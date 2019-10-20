'use strict'

var ethers = require('ethers')

window.Braid = {
    identityFromMnemonic,
    identityFromPrivateKey,
    randomIdentity,
    get,
    put,
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

function get(keypath, cb) {
    setInterval(async () => {
        try {
            var resp = (await (await fetch(keypath, {
                headers: { 'Accept': 'application/json' },
            })).json())
            cb(null, resp)
        } catch (err) {
            cb(err)
        }
    }, 1000)
}

function put(tx, identity) {
    tx.from = identity.address
    tx.sig = identity.signTx(tx)
    return fetch('/', {
        method: 'PUT',
        body: JSON.stringify(tx),
    })
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
