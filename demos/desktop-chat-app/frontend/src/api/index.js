import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'

const sync9JSSha3 = '0c87e1035db28f334cd7484b47d9e7cc285e026d4f876d24ddad78c47ac40a14'

export async function initializeLocalServerRegistry(nodeAddress) {
    let stateURI = 'chat.local/servers'
    let tx = {
        stateURI: stateURI,
        id: Redwood.utils.genesisTxID,
        parents: [],
        patches: [
            ' = ' + Redwood.utils.JSON.stringify({
                'Merge-Type': {
                    'Content-Type': 'resolver/dumb',
                    'value': {},
                },
                'Validator': {
                    'Content-Type': 'validator/permissions',
                    'value': {
                        [nodeAddress]: {
                            '^.*$': {
                                'write': true,
                            },
                        },
                    },
                },
                'value': [],
            }),
        ],
    }
    await rpcFetch('RPC.Subscribe', { StateURI: stateURI })
    await rpcFetch('RPC.SendTx', { Tx: tx })
}

export async function addServer(server, servers) {
    let tx = {
        stateURI: 'chat.local/servers',
        id: Redwood.utils.randomID(),
        patches: [
            `.value[${servers.length}:${servers.length}] = ["${server}"]`,
        ],
    }
    await rpcFetch('RPC.Subscribe', { StateURI: `${server}/registry` })
    await rpcFetch('RPC.SendTx', { Tx: tx })
}

export async function createNewChat(server, newChatName, rooms) {
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
    await rpcFetch('RPC.Subscribe', { StateURI: stateURI })
    console.log('RPC SUBSCRIBE', stateURI)
    await rpcFetch('RPC.SendTx', { Tx: tx })

    tx = {
        stateURI: `${server}/registry`,
        id: Redwood.utils.randomID(),
        patches: [
            `.rooms[${rooms[server].length}:${rooms[server].length}] = ["${stateURI}"]`,
        ],
    }
    await rpcFetch('RPC.SendTx', { Tx: tx })
}

export async function sendMessage(messageText, nodeAddress, server, room, messages) {
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
    await rpcFetch('RPC.SendTx', { Tx: tx })
}