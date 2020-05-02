const ethers = require('ethers')
const stringify = require('json-stable-stringify')

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
    deepmerge,
    JSON: {
        stringify,
    },
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
                if (tx.parents && tx.parents.length > 0) {
                    for (let p of tx.parents) {
                        if (!haveTxs[p]) {
                            missingAParent = true
                            break
                        }
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

function isMergeableObject(val) {
    var nonNullObject = val && typeof val === 'object'

    return nonNullObject
        && Object.prototype.toString.call(val) !== '[object RegExp]'
        && Object.prototype.toString.call(val) !== '[object Date]'
}

function emptyTarget(val) {
    return Array.isArray(val) ? [] : {}
}

function cloneIfNecessary(value, optionsArgument) {
    var clone = optionsArgument && optionsArgument.clone === true
    return (clone && isMergeableObject(value)) ? deepmerge(emptyTarget(value), value, optionsArgument) : value
}

function defaultArrayMerge(target, source, optionsArgument) {
    var destination = target.slice()
    source.forEach(function(e, i) {
        if (typeof destination[i] === 'undefined') {
            destination[i] = cloneIfNecessary(e, optionsArgument)
        } else if (isMergeableObject(e)) {
            destination[i] = deepmerge(target[i], e, optionsArgument)
        } else if (target.indexOf(e) === -1) {
            destination.push(cloneIfNecessary(e, optionsArgument))
        }
    })
    return destination
}

function mergeObject(target, source, optionsArgument) {
    var destination = {}
    if (isMergeableObject(target)) {
        Object.keys(target).forEach(function (key) {
            destination[key] = cloneIfNecessary(target[key], optionsArgument)
        })
    }
    Object.keys(source).forEach(function (key) {
        if (!isMergeableObject(source[key]) || !target[key]) {
            destination[key] = cloneIfNecessary(source[key], optionsArgument)
        } else {
            destination[key] = deepmerge(target[key], source[key], optionsArgument)
        }
    })
    return destination
}

function deepmerge(target, source, optionsArgument) {
    var array = Array.isArray(source);
    var options = optionsArgument || { arrayMerge: defaultArrayMerge }
    var arrayMerge = options.arrayMerge || defaultArrayMerge

    if (array) {
        return Array.isArray(target) ? arrayMerge(target, source, optionsArgument) : cloneIfNecessary(source, optionsArgument)
    } else {
        return mergeObject(target, source, optionsArgument)
    }
}

deepmerge.all = function deepmergeAll(array, optionsArgument) {
    if (!Array.isArray(array) || array.length < 2) {
        throw new Error('first argument should be an array with at least two elements')
    }

    // we are sure there are at least 2 values, so it is safe to have no initial value
    return array.reduce(function(prev, next) {
        return deepmerge(prev, next, optionsArgument)
    })
}
