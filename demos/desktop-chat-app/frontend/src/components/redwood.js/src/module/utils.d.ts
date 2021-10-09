import * as ethers from "ethers";
import stringify from "json-stable-stringify";
import { ResolverFunc, NewStateCallback, NewStateMsg, Tx } from "./types";
declare let genesisTxID: string;
declare let JSON: {
    stringify: typeof stringify;
};
declare let keccak256: typeof ethers.ethers.utils.keccak256;
export { genesisTxID, createTxQueue, hashTx, serializeTx, keccak256, randomID, privateTxRootForRecipients, stringToHex, randomString, hexToUint8Array, uint8ArrayToHex, deepmerge, JSON, };
declare function createTxQueue(resolverFn: ResolverFunc, txProcessedCallback: NewStateCallback): {
    addTx: ({ stateURI, tx, state, leaves }: NewStateMsg) => void;
    defaultTxHandler: (err: string | undefined, { stateURI, tx, state, leaves }: NewStateMsg) => void;
};
declare function hashTx(tx: Tx): string;
declare function serializeTx(tx: Tx): string;
declare function privateTxRootForRecipients(recipients: string[]): string;
declare function randomID(): string;
declare function stringToHex(s: string): string;
declare function randomString(length: number): string;
declare function hexToUint8Array(hexString: string): Uint8Array;
declare function uint8ArrayToHex(bytes: Uint8Array): string;
interface DeepmergeOpts {
    arrayMerge?: typeof defaultArrayMerge;
    clone?: boolean;
}
declare function defaultArrayMerge<T>(target: T[], source: T[], opts?: DeepmergeOpts): any;
declare function deepmerge<A, B>(target: A, source: B, opts?: DeepmergeOpts): A & B;
declare namespace deepmerge {
    var all: (array: any, opts?: DeepmergeOpts | undefined) => any;
}
//# sourceMappingURL=utils.d.ts.map