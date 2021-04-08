import Redwood from 'redwood-p2p-client'

const sync9JSSha3 = '0c87e1035db28f334cd7484b47d9e7cc285e026d4f876d24ddad78c47ac40a14'

export default function(redwoodClient, ownAddress) {
    async function addPeer(transportName, dialAddr) {
        await redwoodClient.rpc.addPeer({ transportName, dialAddr })
    }

    async function addServer(server, iconFile, provider, cloudStackOptions) {
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
                    'rooms': {},
                    'users': {},
                    'iconImg': iconImgPatch
                }),
            ],
        }

        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)

        tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [
                `.value["${server}"] = true`,
            ],
        }
        await redwoodClient.rpc.sendTx(tx)

        if (!!provider && provider !== 'none' && !!cloudStackOptions) {
            await redwoodClient.rpc.rpcFetch('RPC.CreateCloudStack', {
                ...cloudStackOptions,
                firstStateURI: stateURI,
                provider,
            })
        }
    }

    async function importServer(server) {
        let tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [
                `.value["${server}"] = true`,
            ],
        }
        await redwoodClient.rpc.subscribe({ stateURI: `${server}/registry`, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)
    }

    async function subscribe(stateURI) {
        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
    }

    async function createNewChat(server, newChatName) {
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
                                '^.*$': { 'write': true },
                            },
                        },
                    },
                    'messages': [],
                }),
            ],
        }
        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx(tx)

        tx = {
            stateURI: `${server}/registry`,
            id: Redwood.utils.randomID(),
            patches: [
                `.rooms["${newChatName}"] = true`,
            ],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function createNewDM(recipients) {
        let roomName = Redwood.utils.privateTxRootForRecipients(recipients)
        let stateURI = 'chat.p2p/' + roomName

        let users = {}
        for (let addr of recipients) {
            users[addr] = {}
        }

        await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        await redwoodClient.rpc.sendTx({
            stateURI: stateURI,
            id: Redwood.utils.genesisTxID,
            parents: [],
            recipients: recipients,
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
                                '^.*$': { 'write': true },
                            },
                        },
                    },
                    'Members': recipients,
                    'users': users,
                    'messages': [],
                }),
            ],
        })

        await redwoodClient.rpc.sendTx({
            stateURI: 'chat.local/dms',
            id: Redwood.utils.randomID(),
            patches: [
                `.rooms["${roomName}"] = true`,
            ],
        })
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
        await redwoodClient.rpc.sendTx(tx)
    }

    async function updateProfile(address, usersStateURI, username, photoFile) {
        address = address.toLowerCase()
        let patches = []
        if (photoFile) {
            let { sha3 } = await redwoodClient.storeRef(photoFile)
            let { type } = photoFile
            patches.push(`.users.${address}.photo = ` + Redwood.utils.JSON.stringify({
                'Content-Type': type,
                'value': {
                    'Content-Type': 'link',
                    'value': 'ref:sha3:' + sha3,
                },
            }))
        }
        if (username) {
            patches.push(`.users.${address}.username = "${username}"`)
        }
        if (patches.length === 0) {
            return
        }

        let tx = {
            id: Redwood.utils.randomID(),
            stateURI: usersStateURI,
            patches,
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function setNickname(peerAddress, nickname) {
        await redwoodClient.rpc.sendTx({
            id: Redwood.utils.randomID(),
            stateURI: 'chat.local/address-book',
            patches: [
                `.value.${peerAddress} = "${nickname}"`,
            ],
        })
    }

    function createCloudStackOptions(provider, apiKey) {
        return redwoodClient.rpc.rpcFetch('RPC.CreateCloudStackOptions', { provider, apiKey })
    }

    return {
        addPeer,
        addServer,
        importServer,
        subscribe,
        createNewChat,
        createNewDM,
        sendMessage,
        updateProfile,
        setNickname,
        createCloudStackOptions,
    }
}