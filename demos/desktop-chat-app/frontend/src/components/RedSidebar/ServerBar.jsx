import React, { useState, useCallback } from 'react'
import styled from 'styled-components'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import useKnownServers from '../../hooks/useKnownServers'
import useModal from '../../hooks/useModal'
import * as api from '../../api'

import addChat from './assets/add_chat.svg'

const Container = styled.div`
    display: flex;
    flex-direction: column;
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

const ServerIcon = styled.div`
    height: 50px;
    text-align: center;
    padding: 20px 0;
    cursor: pointer;
    font-weight: 700;
    font-size: 1.3rem;
    background: ${props => props.selected ? '#1b203c' : 'transparent'};
    color: white;

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

function ServerBar({ selectedServer, setSelectedServer, className }) {
    const knownServers = useKnownServers()
    const { onPresent: onPresentAddServerModal, onDismiss: onDismissAddServerModal } = useModal('add server')

    const onClickAddServer = useCallback(() => {
        onPresentAddServerModal()
    }, [onPresentAddServerModal, selectedServer])

    return (
        <Container className={className}>
            {knownServers.map(server => (
                <ServerIcon key={server} selected={server === selectedServer} onClick={() => setSelectedServer(server)}>
                    {server.slice(0, 1)}
                </ServerIcon>
            ))}
            <Spacer />
            <SControlWrapper onClick={onClickAddServer}>
                <img src={addChat} alt="Add Server" />
            </SControlWrapper>
            <AddServerModal onDismiss={onDismissAddServerModal} />
        </Container>
    )
}

function AddServerModal({ onDismiss }) {
    const [serverName, setServerName] = useState('')
    const knownServers = useKnownServers()

    function onChangeServerName(e) {
        setServerName(e.target.value)
    }

    async function onClickAdd() {
        await api.addServer(serverName, servers)
    }

    return (
        <Modal modalKey="add server">
            <ModalTitle>Add a server</ModalTitle>
            <ModalContent>
                <Input value={serverName} onChange={onChangeServerName} />
                <Button onClick={onClickAdd}>Add</Button>
            </ModalContent>
            <ModalActions>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

export default ServerBar