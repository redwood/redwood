import React, { useState, useCallback } from 'react'
import styled from 'styled-components'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import * as api from '../../api'

import addChat from './assets/add_chat.svg'
import avatarPlaceholder from './assets/avatar.png'
import placeholder2 from './assets/placeholder2.png'
import placeholder3 from './assets/placeholder3.png'
import placeholder4 from './assets/placeholder4.png'

const ChatBarWrapper = styled.div`
    display: flex;
    flex-direction: column;
    background: #1b203c;
    box-shadow: 0px 3px 3px -2px rgba(0,0,0,0.2), 0px 3px 4px 0px rgba(0,0,0,0.14), 0px 1px 8px 0px rgba(0,0,0,0.12);
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
    &:hover {
        background: #2d3354;
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
    const serverState = useStateTree(`${selectedServer}/registry`)
    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')

    console.log('registry', serverState)

    const onClickCreateNewChat = useCallback(() => {
        if (!selectedServer) {
            return
        }
        onPresentNewChatModal()
    }, [onPresentNewChatModal, selectedServer])

    let serverRooms = (!!serverState ? serverState.rooms : []) || []

    return (
        <ChatBarWrapper className={className}>
            <ChatBarTitle>Groups</ChatBarTitle>
            {serverRooms.map(stateURI => (
                <ChatBarItem
                    stateURI={stateURI}
                    selectedStateURI={selectedStateURI}
                    setSelectedStateURI={setSelectedStateURI}
                />
            ))}

            <Spacer />

            <SControlWrapper onClick={onClickCreateNewChat}>
                <img src={addChat} alt="Add Chat" />
            </SControlWrapper>
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

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    async function onClickCreate() {
        await api.createNewChat(selectedServer, newChatName, serverRooms)
    }
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