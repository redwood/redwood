const Braid = require('../../braidjs/braid-src.js')
const fs = require('fs')
// const watch = require('node-watch')

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
    await genesisUI()
    await genesisChat()
    await genesisCodeEditor()
    // await genesisVideo()
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

// async function genesisVideo() {
//     // Send genesis tx
//     let tx = {
//         stateURI: 'p2pair.local/stream',
//         id: Braid.utils.randomID(),
//         parents: [ prevTxID ],
//         patches: [
//             ' = ' + Braid.utils.JSON.stringify({
//                 'index.html': {
//                     'Content-Type': 'text/html',
//                     'value': {
//                         'Content-Type': 'link',
//                         'value': `ref:sha3:${indexHTMLSha}`,
//                     }
//                 },
//                 'Validator': {
//                     'Content-Type': 'validator/permissions',
//                     'value': {
//                         '*': {
//                             '^.*$': {
//                                 'write': true,
//                             },
//                         },
//                     },
//                 },
//                 'Merge-Type': {
//                     'Content-Type': 'resolver/dumb',
//                     'value': {},
//                 },
//             }),
//         ],
//     }

//     await braidClient.put(tx)

//     let parentTxID = Braid.utils.genesisTxID

//     const upload = _.debounce(async (evt, filename) => {
//         let file = fs.createReadStream(filename)
//         let { sha3: fileSHA } = await braidClient.storeRef(file)

//         try {
//             let indexM3U8 = fs.createReadStream(path.join(__dirname, 'broadcasts', 'live', 'asdf', 'index.m3u8'))
//             let { sha3: indexM3U8SHA } = await braidClient.storeRef(indexM3U8)

//             let txID = Braid.utils.randomID()
//             await braidClient.put({
//                 stateURI: 'redwood.tv/stream-10283',
//                 id: txID,
//                 parents: [ parentTxID ],
//                 patches: [
//                     '["index.m3u8"] = ' + Braid.utils.JSON.stringify({
//                         'Content-Type': 'link',
//                         'value': `ref:sha3:${indexM3U8SHA}`,
//                     }),
//                     `["${path.basename(filename)}"] = ` + Braid.utils.JSON.stringify({
//                         'Content-Type': 'link',
//                         'value': `ref:sha3:${fileSHA}`,
//                     }),
//                 ],
//             })
//             parentTxID = txID

//         } catch (err) {
//             let txID = Braid.utils.randomID()
//             await braidClient.put({
//                 stateURI: 'redwood.tv/stream-10283',
//                 id: txID,
//                 parents: [ parentTxID ],
//                 patches: [
//                     `["${path.basename(filename)}"] = ` + Braid.utils.JSON.stringify({
//                         'Content-Type': 'link',
//                         'value': `ref:sha3:${fileSHA}`,
//                     }),
//                 ],
//             })
//             parentTxID = txID
//         }
//     }, 500)

//     watch('./broadcasts/live/asdf', {}, upload)
// }

main()
