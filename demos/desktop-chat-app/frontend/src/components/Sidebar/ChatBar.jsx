import React, { useState, useCallback, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar } from '@material-ui/core'
import { AddCircleOutline as AddIcon } from '@material-ui/icons'
import moment from 'moment'
import Redwood from 'redwood'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import PeerRow from '../PeerRow'
import { useRedwood, useStateTree } from 'redwood/dist/main/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import useRoomName from '../../hooks/useRoomName'

import addChat from './assets/add_chat.svg'
import avatarPlaceholder from './assets/speech-bubble.svg'

const ChatBarWrapper = styled.div`
    display: flex;
    flex-direction: column;
    background: ${props => props.theme.color.grey[400]};
`

const SGroupItem = styled(GroupItem)`
    cursor: pointer;
    transition: .15s ease-in-out all;

    border-radius: 8px;
    margin: 6px;

    &:hover {
        background: ${props => props.theme.color.grey[300]};
    }
`

const SControlWrapper = styled.div`
    display: flex;
    align-items: center;
    justify-content: start;
    cursor: pointer;
    transition: .15s ease-in-out all;
    background: transparent;
    height: 40px;
    padding-left: 12px;
    color: ${props => props.theme.color.indigo[500]};
    font-weight: 500;

    &:hover {
        background: ${props => props.theme.color.grey[300]};
    }
`

const Spacer = styled.div`
    flex-grow: 1;
`

const SAddIcon = styled(AddIcon)`
    margin-right: 10px;
`

function ChatBar({ className }) {
    const { selectedServer, selectedRoomName, selectedStateURI, registryStateURI, isDirectMessage, navigate } = useNavigation()
    const { stateTrees } = useRedwood()
    const { servers, rooms } = useServerAndRoomInfo()

    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')
    const { onPresent: onPresentNewDMModal, onDismiss: onDismissNewDMModal } = useModal('new dm')
    const theme = useTheme()

    const onClickCreateNewChat = useCallback(() => {
        if (isDirectMessage) {
            onPresentNewDMModal()
        } else {
            onPresentNewChatModal()
        }
    }, [isDirectMessage, onPresentNewChatModal, onPresentNewDMModal])

    const registryState = useStateTree(registryStateURI)
    const serverRooms = Object.keys((!!registryState ? registryState.rooms : {}) || {}).filter(room => !!room).map(room => `${selectedServer}/${room}`)

    return (
        <ChatBarWrapper className={className}>
            {serverRooms.map(roomStateURI => (
                rooms[roomStateURI]
                    ? <ChatBarItem
                        key={roomStateURI}
                        stateURI={roomStateURI}
                        selected={roomStateURI === selectedStateURI}
                        onClick={() => navigate(selectedServer, rooms[roomStateURI].rawName)}
                      />
                    : null
            ))}

            <Spacer />

            {!!selectedServer &&
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <SAddIcon style={{ color: theme.color.indigo[500] }} /> {isDirectMessage ? 'New message' : 'New chat'}
                </SControlWrapper>
            }
            <NewChatModal selectedServer={selectedServer} serverRooms={serverRooms} onDismiss={onDismissNewChatModal} navigate={navigate} />
            <NewDMModal serverRooms={serverRooms} onDismiss={onDismissNewDMModal} navigate={navigate} />
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
        latestTimestamp = chatState.messages[chatState.messages.length - 1].timestamp
        mostRecentMessageText = chatState.messages[chatState.messages.length - 1].text
    }

    let updateTime = useCallback((latestTimestamp) => {
        if (latestTimestamp) {
            let timeAgo = moment.unix(latestTimestamp).fromNow()
            if (timeAgo === 'a few seconds ago') { timeAgo = 'moments ago' }
            setLatestMessageTime(timeAgo)
        }
    }, [setLatestMessageTime])

    useEffect(() => {
        updateTime(latestTimestamp)
    }, [updateTime, latestTimestamp])

    useEffect(() => {
        const intervalID = setInterval(() => updateTime(latestTimestamp), 10000)
        return () => { clearInterval(intervalID) }
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

const SInput = styled(Input)`
    width: 460px;
`

function NewChatModal({ selectedServer, serverRooms, onDismiss, navigate }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()

    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        try {
            await api.createNewChat(selectedServer, newChatName, serverRooms)
            onDismiss()
            navigate(selectedServer, newChatName)
        } catch (err) {
            console.error(err)
        }
    }, [api, selectedServer, newChatName, serverRooms, navigate])

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter') {
            e.stopPropagation()
            onClickCreate()
            setNewChatName('')
        }
    }

    function closeModal() {
      setNewChatName()
      onDismiss()
    }

    return (
        <Modal modalKey="new chat">
            <ModalTitle closeModal={closeModal}>Create a Chat</ModalTitle>
            <ModalContent>
                <Input
                  value={newChatName}
                  onChange={onChangeNewChatName}
                  onKeyDown={onKeyDown}
                  label={'Chat Name'}
                  width={'460px'}
                />
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>Create</Button>
            </ModalActions>
        </Modal>
    )
}

function NewDMModal({ serverRooms, onDismiss, navigate }) {
    const { nodeIdentities } = useRedwood()
    const [sender, setSender] = useState('')
    const api = useAPI()
    let { peersByAddress } = usePeers()
    let peers = Object.keys(peersByAddress).map(addr => peersByAddress[addr]).filter(peer => !peer.isSelf)

    useEffect(() => {
        if (nodeIdentities && nodeIdentities.length > 0) {
            setSender(nodeIdentities[0].address)
        }
    }, [nodeIdentities])

    const onClickCreate = useCallback(async (recipientAddr) => {
        if (!api) { return }
        try {
            await api.createNewDM([sender, recipientAddr])
            onDismiss()
            navigate('chat.p2p', Redwood.utils.privateTxRootForRecipients([sender, recipientAddr]))
        } catch (err) {
            console.error(err)
        }
    }, [api, serverRooms, navigate])

    function onChangeRecipient(e) {
        setRecipient(e.target.value)
    }

    function onChangeSender(e) {
        setSender(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter') {
            e.stopPropagation()
            onClickCreate()
            setRecipient('')
        }
    }

    function closeModal() {
        setRecipient()
        onDismiss()
    }

    return (
        <Modal modalKey="new dm">
            <ModalTitle closeModal={closeModal}>Start a DM</ModalTitle>
            <ModalContent>
                {peers.map(peer => (
                    <PeerRow address={peer.address} onClick={() => onClickCreate(peer.address)} key={peer.address} />
                ))}
            </ModalContent>
        </Modal>
    )
}

export default ChatBar