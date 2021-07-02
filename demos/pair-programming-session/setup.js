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
    await braidClient.put({
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
            }),
        ],
    })
}

async function genesisChat() {
    await braidClient.put({
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
    })
}

async function genesisCodeEditor() {
    let sync9JS = fs.createReadStream('../../braidjs/dist/sync9-otto.js')
    let { sha3: sync9JSSha3 } = await braidClient.storeBlob(sync9JS)

    await braidClient.put({
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
                                'value': `blob:sha3:${sync9JSSha3}`,
                            }
                        }
                    }
                },
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
            }),
        ],
    })
}

async function genesisVideo() {
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
