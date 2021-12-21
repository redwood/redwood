import Redwood from '@redwood.dev/client'

const sync9JSSha3 = 'dd26e14e5768a5359561bbe22aa148f65c68b7cebb33c60905d5969dd97feb92'

export default function(redwoodClient, ownAddress) {
    async function addPeer(transportName, dialAddr) {
        await redwoodClient.rpc.addPeer({ transportName, dialAddr })
    }

    async function addServer(server, iconFile, provider, cloudStackOptions) {
        let stateURI = `${server}/registry`

        let iconImgPatch = null

        if (iconFile) {
          let { sha3 } = await redwoodClient.storeBlob(iconFile)
          let { type } = iconFile

          iconImgPatch = {
            'Content-Type': type,
            'value': {
                'Content-Type': 'link',
                'value': 'blob:sha3:' + sha3,
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
                        'Content-Type': 'resolver/dumb',
                        // 'Content-Type': 'resolver/js',
                        // 'value': {
                        //     'src': {
                        //         'Content-Type': 'link',
                        //         'value': `blob:sha3:${sync9JSSha3}`,
                        //     }
                        // }
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
                    'files': [],
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

        await redwoodClient.rpc.sendTx({
            stateURI: stateURI,
            id: Redwood.utils.genesisTxID,
            parents: [],
            recipients: recipients,
            patches: [
                ' = ' + Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/dumb',
                        // 'Content-Type': 'resolver/js',
                        // 'value': {
                        //     'src': {
                        //         'Content-Type': 'link',
                        //         'value': `blob:sha3:${sync9JSSha3}`,
                        //     }
                        // }
                    },
                    'Validator': {
                        'Content-Type': 'validator/permissions',
                        'value': {
                            '*': {
                                '^.*$': { 'write': true },
                            },
                        },
                    },
                    'Members': users,
                    'users': users,
                    'messages': [],
                    'files': [],
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

        setTimeout(async () => {
            await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        }, 3000)
    }

    async function sendMessage(messageText, files, nodeAddress, server, room, numMessages, numFiles) {
        let now = Math.floor(new Date().getTime() / 1000)
        let patches = []
        let attachments = []
        if (!!files && files.length > 0) {
            attachments = (await Promise.all(
                files.map(file => redwoodClient.storeBlob(file).then(refHashes => ({ refHashes, file })))
            )).map(({ refHashes, file }) => ({
                'Content-Type': file.type,
                'Content-Length': file.size,
                'filename': file.name,
                'value': {
                    'Content-Type': 'link',
                    'value': 'blob:sha3:' + refHashes.sha3,
                },
            }))

            for (let i = 0; i < attachments.length; i++) {
                patches.push(`.files[-0:-0] = ` + Redwood.utils.JSON.stringify([{
                    'Content-Type': attachments[i]['Content-Type'],
                    'Content-Length': attachments[i]['Content-Length'],
                    'filename': attachments[i]['filename'],
                    'value': attachments[i]['value'],
                    'sender': nodeAddress.toLowerCase(),
                    'timestamp': now,
                }]))
            }
        }

        patches.push('.messages[-0:-0] = ' + Redwood.utils.JSON.stringify([{
            sender: nodeAddress.toLowerCase(),
            text: messageText,
            timestamp: now,
            attachments: attachments.length > 0 ? attachments : null,
        }]))


        let tx = {
            id: Redwood.utils.randomID(),
            stateURI: `${server}/${room}`,
            patches: patches,
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function updateProfile(address, usersStateURI, username, photoFile, role) {
        address = address.toLowerCase()
        let patches = []
        if (photoFile) {
            let { sha3 } = await redwoodClient.storeBlob(photoFile)
            let { type } = photoFile
            patches.push(`.users.${address}.photo = ` + Redwood.utils.JSON.stringify({
                'Content-Type': type,
                'value': {
                    'Content-Type': 'link',
                    'value': 'blob:sha3:' + sha3,
                },
            }))
        }
        if (username) {
            patches.push(`.users.${address}.username = "${username}"`)
        }

        if (role) {
          patches.push(`.users.${address}.role = "${role}"`)
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