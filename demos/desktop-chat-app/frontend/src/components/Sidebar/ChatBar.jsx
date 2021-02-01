import React, { useState, useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar } from '@material-ui/core'
import { AddCircleOutline as AddIcon } from '@material-ui/icons'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import useRedwood from '../../hooks/useRedwood'
import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'

import addChat from './assets/add_chat.svg'
import avatarPlaceholder from './assets/user_placeholder.png'

const ChatBarWrapper = styled.div`
    display: flex;
    flex-direction: column;
    background: ${props => props.theme.color.grey[400]};
`

const ChatBarTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    margin-top: 24px;
    margin-bottom: 24px;
    padding-left: 12px;
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
    color: ${props => props.theme.color.green[500]};
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
    const { selectedServer, selectedStateURI, navigate } = useNavigation()
    const { stateTrees } = useRedwood()
    const serverState = useStateTree(!!selectedServer ? `${selectedServer}/registry` : null)
    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')
    const theme = useTheme()

    const onClickCreateNewChat = useCallback(() => {
        if (!selectedServer) {
            return
        }
        onPresentNewChatModal()
    }, [selectedServer, onPresentNewChatModal])

    let serverRooms = ((!!serverState ? serverState.rooms : []) || []).filter(room => !!room)

    return (
        <ChatBarWrapper className={className}>
            <ChatBarTitle>{selectedServer}/</ChatBarTitle>
            {serverRooms.map(roomStateURI => {
                let messages = (stateTrees[roomStateURI] || {}).messages || []
                let mostRecentMessageText
                if (messages.length > 0) {
                    mostRecentMessageText = messages[messages.length - 1].text
                }
                return <ChatBarItem
                            key={roomStateURI}
                            selectedServer={selectedServer}
                            stateURI={roomStateURI}
                            selectedStateURI={selectedStateURI}
                            navigate={navigate}
                            mostRecentMessageText={mostRecentMessageText}
                />
            })}

            <Spacer />

            {!!selectedServer &&
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <SAddIcon style={{ color: theme.color.green[500] }} /> New chat
                </SControlWrapper>
            }
            <NewChatModal selectedServer={selectedServer} serverRooms={serverRooms} onDismiss={onDismissNewChatModal} navigate={navigate} />
        </ChatBarWrapper>
    )
}

function ChatBarItem({ stateURI, selectedServer, selectedStateURI, navigate, mostRecentMessageText }) {
    const chatState = useStateTree(stateURI)
    let [ _, roomName ] = stateURI.split('/')
    return (
        <SGroupItem
            key={stateURI}
            selected={selectedStateURI === stateURI}
            onClick={() => navigate(selectedServer, roomName)}
            name={roomName}
            text={mostRecentMessageText}
            time={'8 min ago'}
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

    return (
        <Modal modalKey="new chat">
            <ModalTitle>Add a chat</ModalTitle>
            <ModalContent>
                <SInput value={newChatName} onChange={onChangeNewChatName} onKeyDown={onKeyDown} />
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>Create</Button>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar