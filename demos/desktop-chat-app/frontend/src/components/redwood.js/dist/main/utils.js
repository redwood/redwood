"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    Object.defineProperty(o, k2, { enumerable: true, get: function() { return m[k]; } });
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || function (mod) {
    if (mod && mod.__esModule) return mod;
    var result = {};
    if (mod != null) for (var k in mod) if (k !== "default" && Object.prototype.hasOwnProperty.call(mod, k)) __createBinding(result, mod, k);
    __setModuleDefault(result, mod);
    return result;
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.JSON = exports.deepmerge = exports.uint8ArrayToHex = exports.hexToUint8Array = exports.randomString = exports.stringToHex = exports.privateTxRootForRecipients = exports.randomID = exports.keccak256 = exports.serializeTx = exports.hashTx = exports.createTxQueue = exports.genesisTxID = void 0;
const ethers = __importStar(require("ethers"));
const json_stable_stringify_1 = __importDefault(require("json-stable-stringify"));
let genesisTxID = "67656e6573697300000000000000000000000000000000000000000000000000";
exports.genesisTxID = genesisTxID;
let JSON = { stringify: json_stable_stringify_1.default };
exports.JSON = JSON;
let keccak256 = ethers.utils.keccak256;
exports.keccak256 = keccak256;
function createTxQueue(resolverFn, txProcessedCallback) {
    let queue = [];
    let haveTxs = {};
    function addTx({ stateURI, tx, state, leaves }) {
        queue.push({ stateURI, tx, state, leaves });
        processQueue();
    }
    function processQueue() {
        while (true) {
            let processedIdxs = [];
            for (let i = 0; i < queue.length; i++) {
                let { stateURI, tx, state, leaves } = queue[i];
                let missingAParent = false;
                if (!!tx && !!tx.parents && tx.parents.length > 0) {
                    for (let p of tx.parents) {
                        if (!haveTxs[p]) {
                            missingAParent = true;
                            break;
                        }
                    }
                }
                if (!missingAParent) {
                    processedIdxs.unshift(i);
                    processTx({ stateURI, tx, state, leaves });
                }
            }
            if (processedIdxs.length === 0) {
                return;
            }
            for (let idx of processedIdxs) {
                queue.splice(idx, 1);
            }
            if (queue.length === 0) {
                return;
            }
        }
    }
    function processTx({ stateURI, tx, state, leaves }) {
        if (!!tx) {
            const newState = resolverFn(tx.from, tx.id, tx.parents, tx.patches);
            haveTxs[tx.id] = true;
            txProcessedCallback({ stateURI, tx, leaves, state: newState });
        }
        else if (!!state) {
            txProcessedCallback({ stateURI, tx, leaves, state });
        }
    }
    return {
        addTx,
        defaultTxHandler: (err, { stateURI, tx, state, leaves }) => {
            if (err)
                throw new Error(err);
            addTx({ stateURI, tx, state, leaves });
        },
    };
}
exports.createTxQueue = createTxQueue;
function hashTx(tx) {
    let txHex = serializeTx(tx);
    return ethers.utils.keccak256(Buffer.from(txHex, "hex")).toString();
}
exports.hashTx = hashTx;
function serializeTx(tx) {
    let txHex = "";
    txHex += tx.id;
    (tx.parents || []).forEach((parent) => (txHex += parent));
    txHex += stringToHex(tx.stateURI);
    tx.patches.forEach((patch) => (txHex += stringToHex(patch)));
    return txHex;
}
exports.serializeTx = serializeTx;
function privateTxRootForRecipients(recipients) {
    return ("private-" +
        ethers.utils
            .keccak256(Buffer.concat(recipients.sort().map((r) => Buffer.from(r, "hex"))))
            .toString()
            .substr(2));
}
exports.privateTxRootForRecipients = privateTxRootForRecipients;
function randomID() {
    return stringToHex(randomString(32));
}
exports.randomID = randomID;
function stringToHex(s) {
    return Buffer.from(s, "utf8").toString("hex");
}
exports.stringToHex = stringToHex;
function randomString(length) {
    let result = "";
    let characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    let charactersLength = characters.length;
    for (let i = 0; i < length; i++) {
        result += characters.charAt(Math.floor(Math.random() * charactersLength));
    }
    return result;
}
exports.randomString = randomString;
function hexToUint8Array(hexString) {
    let result = hexString.match(/.{1,2}/g);
    if (!result) {
        throw new Error(`could not convert "${hexString}" to Uint8Array`);
    }
    return new Uint8Array(result.map((byte) => parseInt(byte, 16)));
}
exports.hexToUint8Array = hexToUint8Array;
function uint8ArrayToHex(bytes) {
    return bytes.reduce((str, byte) => str + byte.toString(16).padStart(2, "0"), "");
}
exports.uint8ArrayToHex = uint8ArrayToHex;
function isMergeableObject(val) {
    let nonNullObject = val && typeof val === "object";
    return (nonNullObject &&
        Object.prototype.toString.call(val) !== "[object RegExp]" &&
        Object.prototype.toString.call(val) !== "[object Date]");
}
function emptyTarget(val) {
    return Array.isArray(val) ? [] : {};
}
function cloneIfNecessary(value, opts) {
    let clone = opts && opts.clone === true;
    return clone && isMergeableObject(value)
        ? deepmerge(emptyTarget(value), value, opts)
        : value;
}
function defaultArrayMerge(target, source, opts) {
    let destination = target.slice();
    source.forEach((e, i) => {
        if (typeof destination[i] === "undefined") {
            destination[i] = cloneIfNecessary(e, opts);
        }
        else if (isMergeableObject(e)) {
            destination[i] = deepmerge(target[i], e, opts);
        }
        else if (target.indexOf(e) === -1) {
            destination.push(cloneIfNecessary(e, opts));
        }
    });
    return destination;
}
function mergeObject(target, source, opts) {
    let destination = {};
    if (isMergeableObject(target)) {
        Object.keys(target).forEach((key) => {
            destination[key] = cloneIfNecessary(target[key], opts);
        });
    }
    Object.keys(source).forEach((key) => {
        if (!isMergeableObject(source[key]) || !target[key]) {
            destination[key] = cloneIfNecessary(source[key], opts);
        }
        else {
            destination[key] = deepmerge(target[key], source[key], opts);
        }
    });
    return destination;
}
function deepmerge(target, source, opts) {
    let array = Array.isArray(source);
    let options = opts || { arrayMerge: defaultArrayMerge };
    let arrayMerge = options.arrayMerge || defaultArrayMerge;
    if (array) {
        return Array.isArray(target)
            ? arrayMerge(target, source, opts)
            : cloneIfNecessary(source, opts);
    }
    else {
        return mergeObject(target, source, opts);
    }
}
exports.deepmerge = deepmerge;
deepmerge.all = function deepmergeAll(array, opts) {
    if (!Array.isArray(array) || array.length < 2) {
        throw new Error("first argument should be an array with at least two elements");
    }
    // we are sure there are at least 2 values, so it is safe to have no initial value
    return array.reduce((prev, next) => {
        return deepmerge(prev, next, opts);
    });
};
//# sourceMappingURL=utils.js.map