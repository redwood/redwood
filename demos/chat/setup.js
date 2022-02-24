const Redwood = require('../../redwood.js').default
const fs = require('fs')

//
// Redwood setup
//
let node1Identity = Redwood.identity.random()
let node1Client = Redwood.createPeer({
    identity: node1Identity,
    httpHost: 'http://localhost:8080',
    rpcEndpoint: 'http://localhost:8081',
    onFoundPeersCallback: (peers) => {}
})

let node2Identity = Redwood.identity.random()
let node2Client = Redwood.createPeer({
    identity: node2Identity,
    httpHost: 'http://localhost:9090',
    rpcEndpoint: 'http://localhost:9091',
    onFoundPeersCallback: (peers) => {}
})

async function main() {
    await node1Client.authorize()
    await node2Client.authorize()
    await genesis()
    console.log('Done.')
    process.exit(0)
}

async function genesis() {
    await node1Client.rpc.subscribe({ stateURI: 'chat.com/room-2837' })
    await node2Client.rpc.subscribe({ stateURI: 'chat.com/room-2837' })

    // Upload our index.html into the state tree so that the HTTP transport will serve it to browsers.
    let indexHTML = fs.createReadStream('./index.html')
    let { sha3: indexHTMLSha3 } = await node1Client.storeBlob(indexHTML)

    // Send the genesis tx (notice that it contains an `index.html` key that references the uploaded file)
    let tx1 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Redwood.utils.JSON.stringify({
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        [node1Identity.address]: {
                            '^.*$': {
                                'write': true
                            }
                        },
                        '*': {
                            '^\\.messages\\.value\\b': {
                                'write': true
                            },
                            '^\\.private-.*': {
                                'write': true
                            },
                        },
                    },
                },
                'messages': {
                    'value': [],
                    // 'Indices': {
                    //     'sender': {
                    //         'Content-Type': 'indexer/keypath',
                    //         'value': {
                    //             'keypath': 'sender'
                    //         }
                    //     }
                    // }
                },
                'index.html': {
                    'Content-Type': 'text/html',
                    'value': {
                        'Content-Type': 'link',
                        'value': `blob:sha3:${indexHTMLSha3}`,
                    }
                }
            }),
        ],
    }
    await node1Client.put(tx1)

    let tx2 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx1.id ],
        patches: [
            '.talk0 = ' + Redwood.utils.JSON.stringify({
            })
        ],
    }
    await node1Client.put(tx2)

    //
    // Now, let's set up a place to store user profiles.  The permissions validator
    // in this part of the tree uses the special ${sender} token, which allows us to
    // declare that any user may only write to the keypath corresponding to their own
    // public key.
    //
    let tx3 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx2.id ],
        patches: [
            '.users = ' + Redwood.utils.JSON.stringify({
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        '*': {
                            '^\\.$(sender)\\.*$': {
                                'write': true
                            }
                        }
                    }
                }
            }),
        ],
    }
    await node1Client.put(tx3)

    // Add a profile for Paul Stamets
    let tx4 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx3.id ],
        patches: [
            `.users.${node1Identity.address.toLowerCase()} = ` + Redwood.utils.JSON.stringify({
                'name': 'Paul Stamets',
                'occupation': 'Astromycologist',
            }),
        ],
    }
    await node1Client.put(tx4)

    // Here, we also add a private portion of Paul Stamets' user profile.  Only he can view this data.
    // Note that this tx contains a `recipients` field, and must use a different `stateURI`.
    let ptx1 = {
        stateURI: 'chat.com/' + Redwood.utils.privateTxRootForRecipients([ node1Identity.address ]),
        id: Redwood.utils.genesisTxID,
        parents: [],
        recipients: [ node1Identity.address ],
        patches: [
            `.profile = ` + Redwood.utils.JSON.stringify({
                'public': {
                    'Content-Type': 'link',
                    'value': `state:chat.com/room-2837/users/${node1Identity.address.toLowerCase()}`,
                },
                'secrets': {
                    'catName': 'Stanley',
                    'favoriteDonutShape': 'toroidal'
                }
            }),
        ],
    }
    await node1Client.put(ptx1)

    //
    // Finally, we add a few initial messages to the chat.
    //
    let tx5 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx4.id ],
        patches: [
            `.messages.value[0:0] = ` + Redwood.utils.JSON.stringify([
                {
                    'text': 'hello!',
                    'sender': node1Identity.address.toLowerCase(),
                }
            ]),
        ],
    }
    await node1Client.put(tx5)

    let tx6 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx5.id ],
        patches: [
            `.messages.value[1:1] = ` + Redwood.utils.JSON.stringify([
                {
                    'text': 'well hello to you too',
                    'sender': node2Identity.address.toLowerCase(),
                }
            ]),
        ],
    }
    await node2Client.put(tx6)

    // This message has an image attachment
    let memeJPG = fs.createReadStream('./meme.jpg')
    let { sha3: memeJPGSha3 } = await node1Client.storeBlob(memeJPG)

    let tx7 = {
        stateURI: 'chat.com/room-2837',
        id: Redwood.utils.randomID(),
        parents: [ tx6.id ],
        patches: [
            `.messages.value[2:2] = ` + Redwood.utils.JSON.stringify([
                {
                    'text': 'who needs a meme?',
                    'sender': node1Identity.address.toLowerCase(),
                    'attachment': {
                        'Content-Type': 'image/jpg',
                        'value': {
                            'Content-Type': 'link',
                            'value': `blob:sha3:${memeJPGSha3}`,
                        },
                    },
                },
            ]),
        ],
    }
    await node1Client.put(tx7)
}

main()
