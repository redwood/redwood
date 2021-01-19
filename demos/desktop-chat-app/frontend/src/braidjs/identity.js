const ethers = require('ethers')
const utils = require('./utils')

module.exports = {
    fromMnemonic,
    fromPrivateKey,
    random,
}

function fromMnemonic(mnemonic) {
    return _constructIdentity(ethers.Wallet.fromMnemonic(mnemonic))
}

function fromPrivateKey(privateKey) {
    if (privateKey.indexOf('0x') !== 0) {
        privateKey = '0x' + privateKey
    }
    return _constructIdentity(new ethers.Wallet(privateKey))
}

function random() {
    return _constructIdentity(ethers.Wallet.createRandom())
}

function _constructIdentity(wallet) {
    var address = wallet.address.slice(2)

    return {
        peerID: utils.randomID(),
        wallet: wallet,
        address: address,
        signTx: async (tx) => {
            const txHash = utils.hashTx(tx)
            const signed = await wallet._signingKey().signDigest(txHash)
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam
        },
        signBytes: async (bytes) => {
            const hash = ethers.utils.keccak256(bytes)
            const signed = await wallet._signingKey().signDigest(hash)
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam
        },
    }
}

