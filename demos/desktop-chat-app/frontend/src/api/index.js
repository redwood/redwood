import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'

const sync9JSSha3 = '0c87e1035db28f334cd7484b47d9e7cc285e026d4f876d24ddad78c47ac40a14'

export default function(redwoodClient) {
    async function addPeer(transportName, dialAddr) {
        await redwoodClient.rpc.addPeer({ TransportName: transportName, DialAddr: dialAddr })
    }

    async function addServer(server, servers, iconFile) {
        let stateURI = `${server}/registry`

        let iconImgPatch = null

        if (iconFile) {
          let { sha3 } = await redwoodClient.storeRef(iconFile)
          let { type } = iconFile

          iconImgPatch = {
            'Content-Type': type,
            'value': {
                'Content-Type': 'link',
                'value': 'ref:sha3:' + sha3,
            },
          }
      }

        let tx = {
            stateURI,
            id: Redwood.utils.genesisTxID,
            patches: [
                ' = ' + Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/dumb',
                        'value': {}
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
                    'rooms': [],
                    'users': {},
                    'iconImg': iconImgPatch
                }),
            ],
        }

        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)

        console.log('API addServer', server, servers)
        tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [
                `.value[${servers.length}:${servers.length}] = ["${server}"]`,
            ],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function importServer(server, servers) {
        console.log('API importServer', server, servers)
        let tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [
                `.value[${servers.length}:${servers.length}] = ["${server}"]`,
            ],
        }
        await redwoodClient.rpc.subscribe({ stateURI: `${server}/registry`, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)
    }

    async function createNewChat(server, newChatName, rooms) {
        console.log('API createNewChat', server, newChatName, rooms)
        let stateURI = `${server}/${newChatName}`
        let tx = {
            stateURI: stateURI,
            id: Redwood.utils.genesisTxID,
            parents: [],
            patches: [
                ' = ' + Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/js',
                        'value': {
                            'src': {
                                'Content-Type': 'link',
                                'value': `ref:sha3:${sync9JSSha3}`,
                            }
                        }
                    },
                    'Validator': {
                        'Content-Type': 'validator/permissions',
                        'value': {
                            '*': {
                                '^\\.messages\\b': { 'write': true },
                            },
                        },
                    },
                    'messages': [],
                }),
            ],
        }
        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)

        rooms = rooms || []

        tx = {
            stateURI: `${server}/registry`,
            id: Redwood.utils.randomID(),
            patches: [
                `.rooms[${rooms.length}:${rooms.length}] = ["${stateURI}"]`,
            ],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function sendMessage(messageText, files, nodeAddress, server, room, messages) {
        let attachments = []
        if (!!files && files.length > 0) {
            attachments = (await Promise.all(
                files.map(file => redwoodClient.storeRef(file).then(refHashes => ({ refHashes, file })))
            )).map(({ refHashes, file }) => ({
                'Content-Type': file.type,
                'Content-Length': file.size,
                'filename': file.name,
                'value': {
                    'Content-Type': 'link',
                    'value': 'ref:sha3:' + refHashes.sha3,
                },
            }))
        }

        let tx = {
            id: Redwood.utils.randomID(),
            stateURI: `${server}/${room}`,
            patches: [
                '.messages[' + messages.length + ':' + messages.length + '] = ' + Redwood.utils.JSON.stringify([{
                    sender: nodeAddress.toLowerCase(),
                    text: messageText,
                    timestamp: Math.floor(new Date().getTime() / 1000),
                    attachments: attachments.length > 0 ? attachments : null,
                }]),
            ],
        }
        console.log('tx', tx)
        await redwoodClient.rpc.sendTx(tx)
    }

    async function updateProfile(nodeAddress, server, username, photoFile) {
        nodeAddress = nodeAddress.toLowerCase()
        let patches = []
        if (photoFile) {
            let { sha3 } = await redwoodClient.storeRef(photoFile)
            let { type } = photoFile
            patches.push(`.users.${nodeAddress}.photo = ` + Redwood.utils.JSON.stringify({
                'Content-Type': type,
                'value': {
                    'Content-Type': 'link',
                    'value': 'ref:sha3:' + sha3,
                },
            }))
        }
        if (username) {
            patches.push(`.users.${nodeAddress}.username = "${username}"`)
        }
        if (patches.length === 0) {
            return
        }

        let tx = {
            id: Redwood.utils.randomID(),
            stateURI: `${server}/registry`,
            patches,
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    return {
        addServer,
        importServer,
        createNewChat,
        sendMessage,
        updateProfile,
    }
}