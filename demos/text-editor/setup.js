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
    await node1Client.rpc.subscribe({ stateURI: 'docs.redwood.dev/document-3192' })
    await node2Client.rpc.subscribe({ stateURI: 'docs.redwood.dev/document-3192' })

    // Upload our index.html into the state tree so that the HTTP transport will serve it to browsers.
    // Also upload the sync9 JS code so that we can use it as our merge resolver.
    let indexHTML = fs.createReadStream('./index.html')
    let sync9JS = fs.createReadStream('../../redwood.js/dist/resolver.sync9.redwood.js')
    let { sha3: indexHTMLSha3 } = await node1Client.storeBlob(indexHTML)
    let { sha3: sync9JSSha3 } = await node1Client.storeBlob(sync9JS)

    // Send the genesis tx (notice that it contains an `index.html` key that references the uploaded file)
    let tx1 = {
        stateURI: 'docs.redwood.dev/document-3192',
        id: Redwood.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Redwood.utils.JSON.stringify({
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
                'index.html': {
                    'Content-Type': 'text/html',
                    'value': {
                        'Content-Type': 'link',
                        'value': `blob:sha3:${indexHTMLSha3}`,
                    }
                },
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {}
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        [node1Identity.address.toLowerCase()]: {
                            '^.*$': {
                                'write': true
                            }
                        },
                        '*': {
                            '^\\.text\\.value.*': {
                                'write': true
                            }
                        }
                    }
                },
            }),
        ],
    }
    await node1Client.put(tx1)
}

main()
