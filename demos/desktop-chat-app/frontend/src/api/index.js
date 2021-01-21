import rpcFetch from '../utils/rpcFetch'
import Redwood from '../redwood.js'

const sync9JSSha3 = '0c87e1035db28f334cd7484b47d9e7cc285e026d4f876d24ddad78c47ac40a14'

export async function createNewChat(newChatName, registry) {
    let stateURI = `chat.redwood.dev/${newChatName}`
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
    await rpcFetch('RPC.SendTx', { Tx: tx })

    tx = {
        stateURI: 'chat.redwood.dev/registry',
        id: Redwood.utils.randomID(),
        patches: [
            `.rooms[${registry.rooms.length}:${registry.rooms.length}] = ["${stateURI}"]`,
        ],
    }
    await rpcFetch('RPC.SendTx', { Tx: tx })
}

export async function sendMessage(stateURI, nodeAddress, appState, messageText) {
    let { messages } = appState[stateURI]
    let tx = {
        id: Redwood.utils.randomID(),
        stateURI: stateURI,
        patches: [
            '.messages[' + messages.length + ':' + messages.length + '] = ' + Redwood.utils.JSON.stringify([{
                sender: nodeAddress.toLowerCase(),
                text: messageText,
            }]),
        ],
    }
    await rpcFetch('RPC.SendTx', { Tx: tx })
}