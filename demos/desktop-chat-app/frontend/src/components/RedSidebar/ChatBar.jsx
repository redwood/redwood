import React, { useState, useCallback } from 'react'
import styled from 'styled-components'
import { Avatar } from '@material-ui/core'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
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
    color: rgba(255, 255, 255, .6);
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
    justify-content: center;
    border-left: 1px solid rgba(255, 255, 255, .12);
    cursor: pointer;
    transition: .15s ease-in-out all;
    background: transparent;
    height: 40px;

    img {
        height: 24px;
        transition: .15s ease-in-out all;
    }

    &:hover {
        background: #2d3354;
        img {
            transform: scale(1.125);
        }
    }
`

const Spacer = styled.div`
    flex-grow: 1;
`

function ChatBar({ selectedServer, selectedStateURI, setSelectedStateURI, className }) {
    const stateURI = selectedServer === null ? null : `${selectedServer}/registry`
    const serverState = useStateTree(stateURI)
    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')

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
            {serverRooms.map(stateURI => (
                <ChatBarItem
                    stateURI={stateURI}
                    selectedStateURI={selectedStateURI}
                    setSelectedStateURI={setSelectedStateURI}
                />
            ))}

            <Spacer />

            {!!selectedServer &&
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <img src={addChat} alt="Add Chat" />
                </SControlWrapper>
            }
            <NewChatModal selectedServer={selectedServer} serverRooms={serverRooms} onDismiss={onDismissNewChatModal} />
        </ChatBarWrapper>
    )
}

function ChatBarItem({ stateURI, selectedStateURI, setSelectedStateURI }) {
    const chatState = useStateTree(stateURI)
    return (
        <SGroupItem
            key={stateURI}
            selected={selectedStateURI === stateURI}
            onClick={() => setSelectedStateURI(stateURI)}
            name={stateURI.split('/')[1]}
            text={`I'm a moon boy that's...`}
            time={'8 min ago'}
            avatar={avatarPlaceholder}
        />
    )
}

function NewChatModal({ selectedServer, serverRooms, onDismiss }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        await api.createNewChat(selectedServer, newChatName, serverRooms)
    }, [api, selectedServer, newChatName, serverRooms])

    return (
        <Modal modalKey="new chat">
            <ModalTitle>Add a chat</ModalTitle>
            <ModalContent>
                <input value={newChatName} onChange={onChangeNewChatName} />
                <button onClick={onClickCreate}>Create</button>
            </ModalContent>
            <ModalActions>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar