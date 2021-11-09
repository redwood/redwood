import React, { useState, useCallback, useEffect } from 'react'
import { useHistory } from 'react-router-dom'
import styled, { useTheme } from 'styled-components'
import { Avatar, Checkbox } from '@material-ui/core'
import { AddCircleOutline as AddIcon, ExitToApp } from '@material-ui/icons'
import { makeStyles, withStyles } from '@material-ui/core/styles'
import moment from 'moment'
import Redwood from '@redwood.dev/client'
import { sortBy } from 'lodash'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input, { InputLabel } from '../Input'
import PeerRow from '../PeerRow'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import useRoomName from '../../hooks/useRoomName'
import useLoginStatus from '../../hooks/useLoginStatus'
import useUsers from '../../hooks/useUsers'
import theme from '../../theme'

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

const SExitToApp = styled(ExitToApp)`
    margin-right: 10px;
`

function ChatBar({ className }) {
    const { selectedServer, selectedRoomName, selectedStateURI, registryStateURI, isDirectMessage, navigate } = useNavigation()
    const { servers, rooms } = useServerAndRoomInfo()
    const { isLoggedIn, logout } = useLoginStatus()
    const { stateTrees } = useRedwood()

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

    const onClickLogout = useCallback(async () => {
        if (!isLoggedIn) { return }
        await logout()
    }, [isLoggedIn, logout])

    const registryState = useStateTree(registryStateURI)
    const serverRooms = Object.keys((!!registryState ? registryState.rooms : {}) || {}).filter(room => !!room).map(room => `${selectedServer}/${room}`)

    return (
        <ChatBarWrapper className={className}>
            {serverRooms.map(roomStateURI => (
                rooms[roomStateURI] && !!stateTrees[roomStateURI]
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
            <SControlWrapper onClick={onClickLogout}>
                <SExitToApp style={{ color: theme.color.indigo[500] }} /> Logout
            </SControlWrapper>
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

	// Put notification sound and alert here

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

const WInput = styled(Input)`
width: 280px;	
`

function NewChatModal({ selectedServer, serverRooms, onDismiss, navigate }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()
    const { rooms } = useServerAndRoomInfo()
    const { nodeIdentities } = useRedwood()

            // console.log(Object.keys(rooms).split('/')[selectedServer])
    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        try {
            await api.createNewChat(selectedServer, newChatName, serverRooms)

            // Check if this is the first room created by user
            const roomsFound = !!Object.keys(rooms).filter((room) => {
              return room.split('/')[0] === selectedServer
            }).length

            if (!roomsFound) {
              await api.updateProfile(nodeIdentities[0].address, `${selectedServer}/registry`, null, null, 'creator')
            }

            setNewChatName('')
            onDismiss()
            navigate(selectedServer, newChatName)
        } catch (err) {
            console.error(err)
        }
    }, [api, selectedServer, newChatName, serverRooms, navigate, rooms, nodeIdentities])

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
				<InputLabel label={'Chat Name'}>
					<WInput
						value={newChatName}
						onChange={onChangeNewChatName}
						onKeyDown={onKeyDown}
					/>
				</InputLabel>
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>Create</Button>
            </ModalActions>
        </Modal>
    )
}

const useCheckboxStyles = makeStyles(muiTheme => ({
    root: {
        color: theme.color.grey[50] + ' !important',
    },
    checked: {
        color: theme.color.green[500] + ' !important',
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
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
`

const SEmptyPeers = styled.div`
	color: rgba(255, 255, 255, .5);
	width: 100%;
	text-align: center;
	font-size: 12px;
	margin-top: 24px;
`

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

    const [selectedPeers, setSelectedPeers] = useState({})
    const onClickPeer = useCallback(async (recipientAddr) => {
        setSelectedPeers({
            ...selectedPeers,
            [recipientAddr]: !selectedPeers[recipientAddr],
        })
    }, [selectedPeers, setSelectedPeers])

    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        try {
            let recipients = Object.keys(selectedPeers).filter(addr => !!selectedPeers[addr])
            await api.createNewDM([...recipients, sender])
            onDismiss()
            navigate('chat.p2p', Redwood.utils.privateTxRootForRecipients(Object.keys(selectedPeers)))
        } catch (err) {
            console.error(err)
        }
        setSelectedPeers({})
    }, [api, serverRooms, navigate, selectedPeers, setSelectedPeers])

    peers = sortBy(peers, ['address'])

	const checkboxStyles = useCheckboxStyles()

	let emptyPeers = null
	let noSelectedPeers = Object.keys(selectedPeers).length === 0

	if (peers.length === 0) {
		emptyPeers = <SEmptyPeers>No peers were found.</SEmptyPeers>
	}

    return (
        <Modal modalKey="new dm">
            <ModalTitle closeModal={onDismiss}>Start a DM</ModalTitle>
            <SNewDMModalContent>
				{emptyPeers}
                {peers.map(peer => (
                    <div style={{ display: 'flex' }} key={peer.address}>
                        <Checkbox checked={!!selectedPeers[peer.address]} onChange={() => onClickPeer(peer.address)} classes={checkboxStyles} />
                        <PeerRow address={peer.address} onClick={() => onClickPeer(peer.address)} />
                    </div>
                ))}
            </SNewDMModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate} disabled={noSelectedPeers}>Create</Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar