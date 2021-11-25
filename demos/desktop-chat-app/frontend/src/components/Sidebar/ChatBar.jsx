import React, { useState, useCallback, useEffect, useRef, useMemo } from 'react'
import styled, { useTheme } from 'styled-components'
import { Checkbox } from '@material-ui/core'
import { AddCircleOutline as AddIcon, ExitToApp } from '@material-ui/icons'
import { makeStyles } from '@material-ui/core/styles'
import moment from 'moment'
import Redwood from '@redwood.dev/client'
import { sortBy } from 'lodash'

import useRedwood from '../../hooks/useRedwood'
import useStateTree from '../../hooks/useStateTree'
import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input, { InputLabel } from '../Input'
import PeerRow from '../PeerRow'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import useRoomName from '../../hooks/useRoomName'
import useLoginStatus from '../../hooks/useLoginStatus'
import mTheme from '../../theme'

import avatarPlaceholder from './assets/speech-bubble.svg'

const ChatBarWrapper = styled.div`
    display: flex;
    flex-direction: column;
    background: ${(props) => props.theme.color.grey[400]};
`

const SGroupItem = styled(GroupItem)`
    cursor: pointer;
    transition: 0.15s ease-in-out all;

    border-radius: 8px;
    margin: 6px;

    &:hover {
        background: ${(props) => props.theme.color.grey[300]};
    }
`

const SControlWrapper = styled.div`
    display: flex;
    align-items: center;
    justify-content: start;
    cursor: pointer;
    transition: 0.15s ease-in-out all;
    background: transparent;
    height: 40px;
    padding-left: 12px;
    color: ${(props) => props.theme.color.indigo[500]};
    font-weight: 500;

    &:hover {
        background: ${(props) => props.theme.color.grey[300]};
    }
`

const Spacer = styled.div`
    flex-grow: 1;
`

const SAddIcon = styled(AddIcon)`
    margin-right: 10px;
    color: ${({ theme }) => theme.color.indigo[500]};
`

const SExitToApp = styled(ExitToApp)`
    margin-right: 10px;
    color: ${({ theme }) => theme.color.indigo[500]};
`

function ChatBar({ className }) {
    const {
        selectedServer,
        selectedStateURI,
        registryStateURI,
        isDirectMessage,
        navigate,
    } = useNavigation()
    const { rooms } = useServerAndRoomInfo()
    const { isLoggedIn, logout } = useLoginStatus()
    const { stateTrees } = useRedwood()

    const {
        onPresent: onPresentNewChatModal,
        onDismiss: onDismissNewChatModal,
    } = useModal('new chat')
    const { onPresent: onPresentNewDMModal, onDismiss: onDismissNewDMModal } =
        useModal('new dm')

    const onClickCreateNewChat = useCallback(() => {
        if (isDirectMessage) {
            onPresentNewDMModal()
        } else {
            onPresentNewChatModal()
        }
    }, [isDirectMessage, onPresentNewChatModal, onPresentNewDMModal])

    const onClickLogout = useCallback(async () => {
        if (!isLoggedIn) {
            return
        }
        await logout()
    }, [isLoggedIn, logout])

    const registryState = useStateTree(registryStateURI)
    const serverRooms = useMemo(
        () =>
            Object.keys((registryState ? registryState.rooms : {}) || {})
                .filter((room) => !!room)
                .map((room) => `${selectedServer}/${room}`),
        [registryState, selectedServer],
    )

    return (
        <ChatBarWrapper className={className}>
            {serverRooms.map((roomStateURI) =>
                rooms[roomStateURI] ? (
                    <ChatBarItem
                        key={roomStateURI}
                        stateURI={roomStateURI}
                        selected={roomStateURI === selectedStateURI}
                        onClick={() =>
                            navigate(
                                selectedServer,
                                rooms[roomStateURI].rawName,
                            )
                        }
                    />
                ) : null,
            )}

            <Spacer />

            {!!selectedServer && (
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <SAddIcon /> {isDirectMessage ? 'New message' : 'New chat'}
                </SControlWrapper>
            )}
            <SControlWrapper onClick={onClickLogout}>
                <SExitToApp /> Logout
            </SControlWrapper>
            <NewChatModal
                selectedServer={selectedServer}
                serverRooms={serverRooms}
                onDismiss={onDismissNewChatModal}
                navigate={navigate}
            />
            <NewDMModal
                serverRooms={serverRooms}
                onDismiss={onDismissNewDMModal}
                navigate={navigate}
            />
        </ChatBarWrapper>
    )
}

function ChatBarItem({ stateURI, selected, onClick }) {
    const chatState = useStateTree(stateURI)
    const [latestMessageTime, setLatestMessageTime] = useState(null)
    const [server, room] = stateURI.split('/')
    const roomName = useRoomName(server, room)

    let latestTimestamp
    let mostRecentMessageText
    if (chatState && chatState.messages && chatState.messages.length > 0) {
        latestTimestamp =
            chatState.messages[chatState.messages.length - 1].timestamp
        mostRecentMessageText =
            chatState.messages[chatState.messages.length - 1].text
    }

    const updateTime = useCallback(
        (timestamp) => {
            if (timestamp) {
                let timeAgo = moment.unix(timestamp).fromNow()
                if (timeAgo === 'a few seconds ago') {
                    timeAgo = 'moments ago'
                }
                setLatestMessageTime(timeAgo)
            }
        },
        [setLatestMessageTime],
    )

    useEffect(() => {
        updateTime(latestTimestamp)
    }, [updateTime, latestTimestamp])

    useEffect(() => {
        const intervalID = setInterval(() => updateTime(latestTimestamp), 10000)
        return () => {
            clearInterval(intervalID)
        }
    }, [latestTimestamp, updateTime])

    return (
        <SGroupItem
            selected={selected}
            onClick={onClick}
            name={roomName}
            text={mostRecentMessageText}
            time={latestMessageTime}
            avatar={avatarPlaceholder}
        />
    )
}

const WInput = styled(Input)`
    width: 280px;
`

function NewChatModal({ selectedServer, serverRooms, onDismiss, navigate }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()
    const { rooms } = useServerAndRoomInfo()
    const { nodeIdentities } = useRedwood()

    const onClickCreate = useCallback(async () => {
        if (!api) {
            return
        }
        try {
            await api.createNewChat(selectedServer, newChatName, serverRooms)

            // Check if this is the first room created by user
            const roomsFound = !!Object.keys(rooms).filter(
                (room) => room.split('/')[0] === selectedServer,
            ).length

            if (!roomsFound) {
                await api.updateProfile(
                    nodeIdentities[0].address,
                    `${selectedServer}/registry`,
                    null,
                    null,
                    'creator',
                )
            }

            setNewChatName('')
            onDismiss()
            navigate(selectedServer, newChatName)
        } catch (err) {
            // NOTE: Need proper error caching here
            throw new Error(err)
        }
    }, [
        api,
        selectedServer,
        newChatName,
        serverRooms,
        navigate,
        rooms,
        nodeIdentities,
    ])

    const onChangeNewChatName = (e) => {
        setNewChatName(e.target.value)
    }

    const onKeyDown = (e) => {
        if (e.code === 'Enter') {
            e.stopPropagation()
            onClickCreate()
            setNewChatName('')
        }
    }

    const inputRef = useRef()

    const onOpen = useCallback(() => {
        if (inputRef.current) {
            inputRef.current.focus()
        }
    }, [])

    const closeModal = () => {
        setNewChatName()
        onDismiss()
    }

    return (
        <Modal modalKey="new chat" onOpen={onOpen} closeModal={closeModal}>
            <ModalTitle closeModal={closeModal}>Create a Chat</ModalTitle>
            <ModalContent>
                <InputLabel label="Chat Name">
                    <WInput
                        ref={inputRef}
                        value={newChatName}
                        onChange={onChangeNewChatName}
                        onKeyDown={onKeyDown}
                    />
                </InputLabel>
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>
                    Create
                </Button>
            </ModalActions>
        </Modal>
    )
}

const useCheckboxStyles = makeStyles(() => ({
    root: {
        color: `${mTheme.color.grey[50]} !important`,
    },
    checked: {
        color: `${mTheme.color.green[500]} !important`,
    },
}))

const SNewDMModalContent = styled(ModalContent)`
    max-height: 60vh;
    overflow: scroll;
    min-width: 400px;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

const SEmptyPeers = styled.div`
    color: rgba(255, 255, 255, 0.5);
    width: 100%;
    text-align: center;
    font-size: 12px;
    margin-top: 24px;
`

function NewDMModal({ serverRooms, onDismiss, navigate }) {
    const { nodeIdentities } = useRedwood()
    const [sender, setSender] = useState('')
    const [selectedPeers, setSelectedPeers] = useState({})
    const api = useAPI()
    const { peersByAddress } = usePeers()

    useEffect(() => {
        if (nodeIdentities && nodeIdentities.length > 0) {
            setSender(nodeIdentities[0].address)
        }
    }, [nodeIdentities])

    let peers = Object.keys(peersByAddress)
        .map((addr) => peersByAddress[addr])
        .filter((peer) => !peer.isSelf)

    const onClickPeer = useCallback(
        async (recipientAddr) => {
            setSelectedPeers({
                ...selectedPeers,
                [recipientAddr]: !selectedPeers[recipientAddr],
            })
        },
        [selectedPeers, setSelectedPeers],
    )

    const onClickCreate = useCallback(async () => {
        if (!api) {
            return
        }
        try {
            const recipients = Object.keys(selectedPeers).filter(
                (addr) => !!selectedPeers[addr],
            )
            await api.createNewDM([...recipients, sender])
            onDismiss()
            navigate(
                'chat.p2p',
                Redwood.utils.privateTxRootForRecipients([
                    sender,
                    /* eslint-disable */
                    recipientAddr, // NOTE: I believe this should be ...recipients
                    /* eslint-enable */
                ]),
            )
        } catch (err) {
            // NOTE: Add proper error caching here
            throw new Error(err)
        }
        setSelectedPeers({})
    }, [api, serverRooms, navigate, selectedPeers, setSelectedPeers])

    peers = sortBy(peers, ['address'])

    const checkboxStyles = useCheckboxStyles()

    let emptyPeers = null
    const noSelectedPeers = Object.keys(selectedPeers).length === 0

    if (peers.length === 0) {
        emptyPeers = <SEmptyPeers>No peers were found.</SEmptyPeers>
    }

    return (
        <Modal modalKey="new dm">
            <ModalTitle closeModal={onDismiss}>Start a DM</ModalTitle>
            <SNewDMModalContent>
                {emptyPeers}
                {peers.map((peer) => (
                    <div style={{ display: 'flex' }} key={peer.address}>
                        <Checkbox
                            checked={!!selectedPeers[peer.address]}
                            onChange={() => onClickPeer(peer.address)}
                            classes={checkboxStyles}
                        />
                        <PeerRow
                            address={peer.address}
                            onClick={() => onClickPeer(peer.address)}
                        />
                    </div>
                ))}
            </SNewDMModalContent>
            <ModalActions>
                <Button
                    primary
                    onClick={onClickCreate}
                    disabled={noSelectedPeers}
                >
                    Create
                </Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar
