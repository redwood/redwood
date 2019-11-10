const ethers = require('ethers')

var genesisTxID = '67656e6573697300000000000000000000000000000000000000000000000000'


module.exports = {
    genesisTxID,

    createTxQueue,
    hashTx,
    serializeTx,
    randomID,
    stringToHex,
    randomString,
    hexToUint8Array,
    uint8ArrayToHex,
}



function createTxQueue(resolverFn, txProcessedCallback) {
    let queue = []
    let haveTxs = {}

    function addTx(tx) {
        queue.push(tx)
        processQueue()
    }

    function processQueue() {
        while (true) {
            let processedIdxs = []
            for (let i = 0; i < queue.length; i++) {
                let tx = queue[i]
                let missingAParent = false
                for (let p of tx.parents) {
                    if (!haveTxs[p]) {
                        missingAParent = true
                        break
                    }
                }
                if (!missingAParent) {
                    processedIdxs.unshift(i)
                    processTx(tx)
                }
            }

            if (processedIdxs.length === 0) {
                return
            }

            for (let idx of processedIdxs) {
                queue.splice(idx, 1)
            }

            if (queue.length === 0) {
                return
            }
        }
    }

    function processTx(tx) {
        const newState = resolverFn(tx.from, tx.id, tx.parents, tx.patches)
        haveTxs[tx.id] = true
        txProcessedCallback(tx, newState)
    }

    return {
        addTx,
        defaultTxHandler: (err, tx) => {
            if (err) throw new Error(err)
            addTx(tx)
        },
    }
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
