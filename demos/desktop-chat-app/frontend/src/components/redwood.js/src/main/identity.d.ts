import { Wallet as EthersWallet, utils as ethersUtils } from 'ethers';
import { Tx } from './types';
export { fromMnemonic, fromPrivateKey, random, };
declare function fromMnemonic(mnemonic: string): {
    peerID: string;
    wallet: EthersWallet;
    address: string;
    signTx: (tx: Tx) => string;
    signBytes: (bytes: ethersUtils.BytesLike) => string;
};
declare function fromPrivateKey(privateKey: string): {
    peerID: string;
    wallet: EthersWallet;
    address: string;
    signTx: (tx: Tx) => string;
    signBytes: (bytes: ethersUtils.BytesLike) => string;
};
declare function random(): {
    peerID: string;
    wallet: EthersWallet;
    address: string;
    signTx: (tx: Tx) => string;
    signBytes: (bytes: ethersUtils.BytesLike) => string;
};
//# sourceMappingURL=identity.d.ts.map