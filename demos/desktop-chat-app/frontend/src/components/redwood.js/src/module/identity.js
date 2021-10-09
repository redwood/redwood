import { Wallet as EthersWallet, utils as ethersUtils } from 'ethers';
import * as utils from './utils';
export { fromMnemonic, fromPrivateKey, random, };
function fromMnemonic(mnemonic) {
    return _constructIdentity(EthersWallet.fromMnemonic(mnemonic));
}
function fromPrivateKey(privateKey) {
    if (privateKey.indexOf('0x') !== 0) {
        privateKey = '0x' + privateKey;
    }
    return _constructIdentity(new EthersWallet(privateKey));
}
function random() {
    return _constructIdentity(EthersWallet.createRandom());
}
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
            const hash = ethersUtils.keccak256(bytes);
            const signed = wallet._signingKey().signDigest(hash);
            return signed.r.slice(2) + signed.s.slice(2) + '0' + signed.recoveryParam;
        },
    };
}
