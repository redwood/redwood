const Braid = require('../../braidjs/braid-src.js')
const fs = require('fs')

//
// Braid setup
//
let braidClient = Braid.createPeer({
    identity: Braid.identity.random(),
    httpHost: 'http://localhost:8080',
    onFoundPeersCallback: (peers) => {},
})

async function main() {
    await braidClient.authorize()
    await genesis()
}

async function genesis() {
    // Upload HTML pages
    let indexHTML = fs.createReadStream('./index.html')
    let recordHTML = fs.createReadStream('./record.html')
    let playHTML = fs.createReadStream('./play.html')
    let { sha3: indexHTMLSha3 } = await braidClient.storeRef(indexHTML)
    let { sha3: recordHTMLSha3 } = await braidClient.storeRef(recordHTML)
    let { sha3: playHTMLSha3 } = await braidClient.storeRef(playHTML)

    // Send genesis tx
    let tx = {
        stateURI: 'redwood.tv/stream-10283',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'index.html': {
                    'Content-Type': 'text/html',
                    'value': {
                        'Content-Type': 'link',
                        'value': `blob:sha3:${indexHTMLSha3}`,
                    }
                },
                'record.html': {
                    'Content-Type': 'text/html',
                    'value': {
                        'Content-Type': 'link',
                        'value': `blob:sha3:${recordHTMLSha3}`,
                    }
                },
                'play.html': {
                    'Content-Type': 'text/html',
                    'value': {
                        'Content-Type': 'link',
                        'value': `blob:sha3:${playHTMLSha3}`,
                    }
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^.*$': {
                                'write': true,
                            },
                        },
                    },
                },
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {},
                },
            }),
        ],
    }
    await braidClient.put(tx)
}

main()
