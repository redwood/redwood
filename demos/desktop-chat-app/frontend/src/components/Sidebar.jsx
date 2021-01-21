import React, { useState } from 'react'
import styled from 'styled-components'
import useBraid from '../hooks/useBraid'
import * as api from '../api'

const SSidebar = styled.div`
    display: flex;
    flex-direction: column;
    height: calc(100vh - 40px);
    padding: 20px 0;
`

const SidebarTitle = styled.div`
    font-weight: 700;
    font-size: 1.4rem;
    margin-left: 10px;
    margin-bottom: 10px;
`

const SidebarItem = styled.div`
    background-color: ${props => props.active ? 'grey' : 'unset'};
    cursor: pointer;
    padding: 10px 20px 10px 26px;
`

const ChatName = styled.div`
    font-weight: 500;
`

const Spacer = styled.div`
    flex-grow: 1;
`

const CreateChatControls = styled.div`
    display: flex;
    flex-direction: column;
`

function Sidebar({ selectedStateURI, setSelectedStateURI, className }) {
    const { appState, registry } = useBraid()
    const [newChatName, setNewChatName] = useState('')

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    function onClickChat(stateURI) {
        setSelectedStateURI(stateURI)
    }

    async function onClickCreateNewChat() {
        await api.createNewChat(newChatName, registry)
    }

    let stateURIs = Object.keys(appState)

    return (
        <SSidebar className={className}>
            <SidebarTitle>Chats</SidebarTitle>

            {(stateURIs || []).map(stateURI => (
                <SidebarItem key={stateURI} active={stateURI === selectedStateURI} onClick={() => onClickChat(stateURI)}>
                    <ChatName>{stateURI.slice('chat.redwood.dev/'.length)}</ChatName>
                </SidebarItem>
            ))}
            <Spacer />
            <CreateChatControls>
                <input onChange={onChangeNewChatName} value={newChatName} />
                <button onClick={onClickCreateNewChat}>Create new chat</button>
            </CreateChatControls>
        </SSidebar>
    )
}

export default Sidebar
