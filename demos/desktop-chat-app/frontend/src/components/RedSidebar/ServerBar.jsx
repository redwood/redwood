import React, { useState, useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, Fab, IconButton } from '@material-ui/core'
import { Add as AddIcon } from '@material-ui/icons'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    padding: ${props => props.verticalPadding} 0;
`

const ServerIconWrapper = styled.div`
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 8px 0;
    cursor: pointer;
    text-transform: uppercase;
    transition: .12s ease-in-out all;

    &:hover {
        // background: #2d3354;
        img {
            transform: scale(1.125);
        }
    }
`

const ServerIcon = styled(Button)`
    border-radius: 9999px;
`

const CircularButton = styled(IconButton)`
    border-radius: 9999;
    background-color: ${props => props.theme.color.grey[200]} !important;
`

const Spacer = styled.div`
    flex-grow: 1;
`

const SFab = styled(Fab)`
    width: 50px !important;
    height: 50px !important;
    transition: .12s ease-in-out border-radius !important;
    background-color: ${props => props.theme.color.grey[400]} !important;
    color: ${props => props.theme.color.white};

    &:hover {
        border-radius: 20px !important;
    }
`

function ServerBar({ selectedServer, setSelectedServer, className, verticalPadding }) {
    const knownServersTree = useStateTree('chat.local/servers')
    const { onPresent: onPresentAddServerModal, onDismiss: onDismissAddServerModal } = useModal('add server')
    const theme = useTheme()

    const onClickAddServer = useCallback(() => {
        onPresentAddServerModal()
    }, [onPresentAddServerModal, selectedServer])

    let knownServers = (knownServersTree || {}).value || []

    return (
        <Container className={className} verticalPadding={verticalPadding}>
            {knownServers.map(server => (
                <ServerIconWrapper key={server} selected={server === selectedServer} onClick={() => setSelectedServer(server)}>
                    <SFab>{server.slice(0, 1)}</SFab>
                </ServerIconWrapper>
            ))}
            <Spacer />
            <ServerIconWrapper onClick={onClickAddServer}>
                <CircularButton><AddIcon style={{ color: theme.color.green[500] }} /></CircularButton>
            </ServerIconWrapper>
            <AddServerModal onDismiss={onDismissAddServerModal} />
        </Container>
    )
}

function AddServerModal({ onDismiss }) {
    const [serverName, setServerName] = useState('')
    const api = useAPI()
    const knownServersTree = useStateTree('chat.local/servers')

    function onChangeServerName(e) {
        setServerName(e.target.value)
    }

    let knownServers = (knownServersTree || {}).value || []

    const onClickAdd = useCallback(async () => {
        if (!api) { return }
        await api.addServer(serverName, knownServers)
    }, [api, serverName, knownServers])

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