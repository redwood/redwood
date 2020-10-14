const Braid = require('../../braidjs/braid-src.js')
const fs = require('fs')
const yaml = require('js-yaml')

//
// Braid setup
//
const mnemonic = yaml.safeLoad(fs.readFileSync('./node1.redwoodrc', 'utf8')).Node.HDMnemonicPhrase
let braidClient = Braid.createPeer({
    identity: Braid.identity.fromMnemonic(mnemonic),
    httpHost: 'http://localhost:8080',
    onFoundPeersCallback: (peers) => {},
})

async function main() {
    await braidClient.authorize()
    await genesisUI()
    await genesisChat()
    await genesisCodeEditor()
    await genesisVideo()
}

async function genesisUI() {
    // let indexHTML = fs.createReadStream('./index.html')
    // let { sha3: indexHTMLSha3 } = await braidClient.storeRef(indexHTML)

    let tx = {
        stateURI: 'p2pair.local/ui',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^.*$': {
                                'write': true
                            }
                        },
                    },
                },
                // 'index.html': {
                //     'Content-Type': 'text/html',
                //     'value': {
                //         'Content-Type': 'link',
                //         'value': `ref:sha3:${indexHTMLSha3}`,
                //     }
                // }
            }),
        ],
    }
    await braidClient.put(tx)
}

async function genesisChat() {
    // Send the genesis tx (notice that it contains an `index.html` key that references the uploaded file)
    let tx = {
        stateURI: 'p2pair.local/chat',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^.*$': {
                                'write': true
                            }
                        },
                    },
                },
                'messages': {
                    'value': [],
                },
                'users': {
                    'Validator': {
                        'Content-Type': 'validator/permissions',
                        'value': {
                            '*': {
                                '^\\.$(sender).*$': {
                                    'write': true
                                },
                            },
                        },
                    },
                },
            }),
        ],
    }
    await braidClient.put(tx)
}

async function genesisCodeEditor() {
    let sync9JS = fs.createReadStream('../../braidjs/dist/sync9-otto.js')
    let { sha3: sync9JSSha3 } = await braidClient.storeRef(sync9JS)

    // Send the genesis tx (notice that it contains an `index.html` key that references the uploaded file)
    let tx = {
        stateURI: 'p2pair.local/editor',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'text': {
                    'value': '',

                    'Merge-Type': {
                        'Content-Type': 'resolver/js',
                        'value': {
                            'src': {
                                'Content-Type': 'link',
                                'value': `ref:sha3:${sync9JSSha3}`,
                            }
                        }
                    }
                },
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                // 'Validator': {
                //     'Content-Type': 'validator/permissions',
                //     'value': {
                //         '*': {
                //             '*': {
                //                 'write': true
                //             }
                //         }
                //     }
                // },
            }),
        ],
    }
    await braidClient.put(tx)
}

async function genesisVideo() {
    // Send genesis tx
    await braidClient.put({
        stateURI: 'p2pair.local/video',
        id: Braid.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Braid.utils.JSON.stringify({
                'streams': {},
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^.*$': { 'write': true },
                        },
                    },
                },
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {},
                },
            }),
        ],
    })
}

main()
