import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'

const sync9JSSha3 = '0c87e1035db28f334cd7484b47d9e7cc285e026d4f876d24ddad78c47ac40a14'

export default function(redwoodClient) {
    async function addServer(server, servers) {
        let stateURI = `${server}/registry`
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

    async function sendMessage(messageText, nodeAddress, server, room, messages) {
        console.log('API sendMessage', { messageText, nodeAddress, server, room, messages })
        let tx = {
            id: Redwood.utils.randomID(),
            stateURI: `${server}/${room}`,
            patches: [
                '.messages[' + messages.length + ':' + messages.length + '] = ' + Redwood.utils.JSON.stringify([{
                    sender: nodeAddress.toLowerCase(),
                    text: messageText,
                }]),
            ],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    return {
        addServer,
        createNewChat,
        sendMessage,
    }
}