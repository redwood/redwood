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

function ChatBar({ selectedServer, selectedStateURI, setSelectedStateURI, className }) {
    const stateURI = selectedServer === null ? null : `${selectedServer}/registry`
    const { stateTrees } = useRedwood()
    const serverState = useStateTree(stateURI)
    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')
    const theme = useTheme()

    const onClickCreateNewChat = useCallback(() => {
        if (!stateURI || !selectedServer) {
            return
        }
        onPresentNewChatModal()
    }, [stateURI, onPresentNewChatModal, selectedServer])

    let serverRooms = ((!!serverState ? serverState.rooms : []) || []).filter(room => !!room)

    return (
        <ChatBarWrapper className={className}>
            <ChatBarTitle>{selectedServer}/</ChatBarTitle>
            {serverRooms.map(stateURI => {
                let messages = (stateTrees[stateURI] || {}).messages || []
                console.log('tree', stateTrees[stateURI])
                let mostRecentMessageText
                if (messages.length > 0) {
                    mostRecentMessageText = messages[messages.length - 1].text
                }
                return <ChatBarItem
                            stateURI={stateURI}
                            mostRecentMessageText={mostRecentMessageText}
                            selectedStateURI={selectedStateURI}
                            setSelectedStateURI={setSelectedStateURI}
                />
            })}

            <Spacer />

            {!!selectedServer &&
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <SAddIcon style={{ color: theme.color.green[500] }} /> New chat
                </SControlWrapper>
            }
            <NewChatModal selectedServer={selectedServer} serverRooms={serverRooms} onDismiss={onDismissNewChatModal} setSelectedStateURI={setSelectedStateURI} />
        </ChatBarWrapper>
    )
}

function ChatBarItem({ stateURI, mostRecentMessageText, selectedStateURI, setSelectedStateURI }) {
    const chatState = useStateTree(stateURI)
    return (
        <SGroupItem
            key={stateURI}
            selected={selectedStateURI === stateURI}
            onClick={() => setSelectedStateURI(stateURI)}
            name={stateURI.split('/')[1]}
            text={mostRecentMessageText}
            time={'8 min ago'}
            avatar={avatarPlaceholder}
        />
    )
}

function NewChatModal({ selectedServer, serverRooms, onDismiss, setSelectedStateURI }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()

    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        try {
            await api.createNewChat(selectedServer, newChatName, serverRooms)
            onDismiss()
        } catch (err) {
            console.error(err)
        }
    }, [api, selectedServer, newChatName, serverRooms])

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter') {
            e.stopPropagation()
            onClickCreate()
            setSelectedStateURI(`${selectedServer}/${newChatName}`)
            setNewChatName('')
        }
    }

    return (
        <Modal modalKey="new chat">
            <ModalTitle>Add a chat</ModalTitle>
            <ModalContent>
                <Input value={newChatName} onChange={onChangeNewChatName} onKeyDown={onKeyDown} />
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>Create</Button>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar