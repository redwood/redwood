import Redwood from '@redwood.dev/client'

const sync9JSSha3 =
    'dd26e14e5768a5359561bbe22aa148f65c68b7cebb33c60905d5969dd97feb92'

const api = (redwoodClient, ownAddress) => {
    async function addPeer(transportName, dialAddr) {
        await redwoodClient.rpc.addPeer({ transportName, dialAddr })
    }

    async function addServer(server, iconFile, provider, cloudStackOptions) {
        const stateURI = `${server}/registry`

        let iconImgPatch = null

        if (iconFile) {
            const { sha3 } = await redwoodClient.storeBlob(iconFile)
            const { type } = iconFile

            iconImgPatch = {
                'Content-Type': type,
                value: {
                    'Content-Type': 'link',
                    value: `blob:sha3:${sha3}`,
                },
            }
        }

        let tx = {
            stateURI,
            id: Redwood.utils.genesisTxID,
            patches: [
                ` = ${Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/dumb',
                        value: {},
                    },
                    Validator: {
                        'Content-Type': 'validator/permissions',
                        value: {
                            '*': {
                                '^.*$': {
                                    write: true,
                                },
                            },
                        },
                    },
                    rooms: {},
                    users: {},
                    iconImg: iconImgPatch,
                })}`,
            ],
        }

        await redwoodClient.rpc.subscribe({
            stateURI,
            keypath: '/',
            txs: true,
            states: true,
        })
        await redwoodClient.rpc.sendTx(tx)

        tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [`.value["${server}"] = true`],
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
        const tx = {
            stateURI: 'chat.local/servers',
            id: Redwood.utils.randomID(),
            patches: [`.value["${server}"] = true`],
        }
        await redwoodClient.rpc.subscribe({
            stateURI: `${server}/registry`,
            keypath: '/',
            txs: true,
            states: true,
        })
        await redwoodClient.rpc.sendTx(tx)
    }

    async function subscribe(stateURI) {
        await redwoodClient.rpc.subscribe({
            stateURI,
            keypath: '/',
            txs: true,
            states: true,
        })
    }

    async function createNewChat(server, newChatName) {
        const stateURI = `${server}/${newChatName}`
        let tx = {
            stateURI,
            id: Redwood.utils.genesisTxID,
            parents: [],
            patches: [
                ` = ${Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/js',
                        value: {
                            src: {
                                'Content-Type': 'link',
                                value: `blob:sha3:${sync9JSSha3}`,
                            },
                        },
                    },
                    Validator: {
                        'Content-Type': 'validator/permissions',
                        value: {
                            '*': {
                                '^.*$': { write: true },
                            },
                        },
                    },
                    messages: [],
                })}`,
            ],
        }
        await redwoodClient.rpc.subscribe({
            stateURI,
            keypath: '/',
            txs: true,
            states: true,
        })
        await redwoodClient.rpc.sendTx(tx)

        tx = {
            stateURI: `${server}/registry`,
            id: Redwood.utils.randomID(),
            patches: [`.rooms["${newChatName}"] = true`],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function createNewDM(recipients) {
        const roomName = Redwood.utils.privateTxRootForRecipients(recipients)
        const stateURI = `chat.p2p/${roomName}`

        const users = {}
        for (const addr of recipients) {
            users[addr] = {}
        }

        await redwoodClient.rpc.subscribe({
            stateURI,
            keypath: '/',
            txs: true,
            states: true,
        })
        await redwoodClient.rpc.sendTx({
            stateURI,
            id: Redwood.utils.genesisTxID,
            parents: [],
            recipients,
            patches: [
                ` = ${Redwood.utils.JSON.stringify({
                    'Merge-Type': {
                        'Content-Type': 'resolver/js',
                        value: {
                            src: {
                                'Content-Type': 'link',
                                value: `blob:sha3:${sync9JSSha3}`,
                            },
                        },
                    },
                    Validator: {
                        'Content-Type': 'validator/permissions',
                        value: {
                            '*': {
                                '^.*$': { write: true },
                            },
                        },
                    },
                    Members: users,
                    users,
                    messages: [],
                })}`,
            ],
        })

        await redwoodClient.rpc.sendTx({
            stateURI: 'chat.local/dms',
            id: Redwood.utils.randomID(),
            patches: [`.rooms["${roomName}"] = true`],
        })

        setTimeout(async () => {
            await redwoodClient.rpc.subscribe({ stateURI, keypath: '/', txs: true, states: true })
        }, 3000)
    }

    async function sendMessage(
        messageText,
        files,
        nodeAddress,
        server,
        room,
        messages,
    ) {
        let attachments = []
        if (!!files && files.length > 0) {
            attachments = (
                await Promise.all(
                    files.map((file) =>
                        redwoodClient
                            .storeBlob(file)
                            .then((refHashes) => ({ refHashes, file })),
                    ),
                )
            ).map(({ refHashes, file }) => ({
                'Content-Type': file.type,
                'Content-Length': file.size,
                filename: file.name,
                value: {
                    'Content-Type': 'link',
                    value: `blob:sha3:${refHashes.sha3}`,
                },
            }))
        }

        const tx = {
            id: Redwood.utils.randomID(),
            stateURI: `${server}/${room}`,
            patches: [
                `.messages[${messages.length}:${
                    messages.length
                }] = ${Redwood.utils.JSON.stringify([
                    {
                        sender: nodeAddress.toLowerCase(),
                        text: messageText,
                        timestamp: Math.floor(new Date().getTime() / 1000),
                        attachments:
                            attachments.length > 0 ? attachments : null,
                    },
                ])}`,
            ],
        }
        await redwoodClient.rpc.sendTx(tx)
    }

    async function updateProfile(
        rAddress,
        usersStateURI,
        username,
        photoFile,
        role,
    ) {
        const address = rAddress.toLowerCase()
        const patches = []
        if (photoFile) {
            const { sha3 } = await redwoodClient.storeBlob(photoFile)
            const { type } = photoFile
            patches.push(
                `.users.${address}.photo = ${Redwood.utils.JSON.stringify({
                    'Content-Type': type,
                    value: {
                        'Content-Type': 'link',
                        value: `blob:sha3:${sha3}`,
                    },
                })}`,
            )
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

        const tx = {
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
            patches: [`.value.${peerAddress} = "${nickname}"`],
        })
    }

    function createCloudStackOptions(provider, apiKey) {
        return redwoodClient.rpc.rpcFetch('RPC.CreateCloudStackOptions', {
            provider,
            apiKey,
        })
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

export default api
