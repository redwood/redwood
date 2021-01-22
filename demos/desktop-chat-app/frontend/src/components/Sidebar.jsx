import React, { useState } from 'react'
import styled from 'styled-components'
import useRedwood from '../hooks/useRedwood'
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
    const { appState, rooms } = useRedwood()
    const [newChatName, setNewChatName] = useState('')

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    function onClickChat(stateURI) {
        setSelectedStateURI(stateURI)
    }

    async function onClickCreateNewChat() {
        await api.createNewChat(newChatName, rooms)
    }

    let stateURIs = Object.keys(appState)
    let domains = {}
    for (let stateURI of stateURIs) {
        let parts = stateURI.split('/')
        let domain = parts[0]
        domains[domain] = domains[domain] || []
        domains[domain].push(stateURI)
    }

    return (
        <SSidebar className={className}>
            <SidebarTitle>Chats</SidebarTitle>

            {(domains || []).map(domain =>
                {(stateURIs || []).map(stateURI => (
                    <SidebarItem key={stateURI} active={stateURI === selectedStateURI} onClick={() => onClickChat(stateURI)}>
                        <ChatName>{stateURI.slice(stateURI.indexOf('/') + 1)}</ChatName>
                    </SidebarItem>
                ))}
            )}
            <Spacer />
            <CreateChatControls>
                <input onChange={onChangeNewChatName} value={newChatName} />
                <button onClick={onClickCreateNewChat}>Create new chat</button>
            </CreateChatControls>
        </SSidebar>
    )
}

export default Sidebar
