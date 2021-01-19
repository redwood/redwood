//
// This is just notes for later
//
var elliptic = require('elliptic')
var EC = elliptic.ec
var ec = new EC('secp256k1')
var key = ec.genKeyPair()
var key = ec.keyFromPrivate(new Uint8Array(arrayify('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')))
var bufX = key.getPublic().getX().toArrayLike(Uint8Array)
var bufY = key.getPublic().getY().toArrayLike(Uint8Array)
var concatted = new Uint8Array(new ArrayBuffer(bufX.length + bufY.length))
concatted.set(bufX, 0)
concatted.set(bufY, bufX.length)
var hexStr = ''
for (var i = 0; i < concatted.length; i++) {
    hexStr += concatted[i].toString(16)
}
var address = ethUtil.keccak256(hexStr).toString('hex')

// Sign the tx (input must be an array, or a hex-string)
var sig = key.sign(txHash, {canonical: true})
console.log('sig', sig)
console.log('r-', sig.r.toString(16))
console.log('r2', normalizeHex(sig.r.toString(16), 32))
console.log('s-', sig.s.toString(16))
console.log('s2', normalizeHex(sig.s.toString(16), 32))
return {
    r: normalizeHex(sig.r.toString(16), 32),
    s: normalizeHex(sig.s.toString(16), 32),
    v: sig.recoveryParam ? '1c': '1b' //normalizeHex(sig.recoveryParam.toString(16), 1),
}

return sig



function arrayify(value) {
    value = value.substring(2);
    if (value.length % 2) { value = '0' + value; }

    var result = [];
    for (var i = 0; i < value.length; i += 2) {
        result.push(parseInt(value.substr(i, 2), 16));
    }

    return new Uint8Array(result);
}
