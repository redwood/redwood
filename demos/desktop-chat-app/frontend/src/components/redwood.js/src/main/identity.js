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
Object.defineProperty(exports, "__esModule", { value: true });
exports.random = exports.fromPrivateKey = exports.fromMnemonic = void 0;
const ethers_1 = require("ethers");
const utils = __importStar(require("./utils"));
function fromMnemonic(mnemonic) {
    return _constructIdentity(ethers_1.Wallet.fromMnemonic(mnemonic));
}
exports.fromMnemonic = fromMnemonic;
function fromPrivateKey(privateKey) {
    if (privateKey.indexOf('0x') !== 0) {
        privateKey = '0x' + privateKey;
    }
    return _constructIdentity(new ethers_1.Wallet(privateKey));
}
exports.fromPrivateKey = fromPrivateKey;
function random() {
    return _constructIdentity(ethers_1.Wallet.createRandom());
}
exports.random = random;
function _constructIdentity(wallet) {
    const address = wallet.address.slice(2);
    return {
        peerID: utils.randomID(),
        wallet: wallet,
        address: address,
        signTx: (tx) => {
            const txHash = utils.hashTx(tx);
            const signed = wallet._signingKey().signDigest(txHash);
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam;
        },
        signBytes: (bytes) => {
            const hash = ethers_1.utils.keccak256(bytes);
            const signed = wallet._signingKey().signDigest(hash);
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam;
        },
    };
}
