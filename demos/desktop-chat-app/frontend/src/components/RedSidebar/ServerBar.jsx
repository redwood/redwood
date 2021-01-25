import React, { useState, useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, Fab, IconButton } from '@material-ui/core'
import { Add as AddIcon, CloudDownloadRounded as ImportIcon } from '@material-ui/icons'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import strToColor from '../../utils/strToColor'

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
    background-color: ${props => strToColor(props.text)} !important;
    color: ${props => props.theme.color.white} !important;
    font-weight: 700 !important;
    font-size: 1.1rem !important;

    &:hover {
        border-radius: 20px !important;
    }
`

const PrimaryButton = styled(Button)`
    background-color: ${props => props.theme.color.green} !important;
`

function ServerBar({ selectedServer, setSelectedServer, className, verticalPadding }) {
    const knownServersTree = useStateTree('chat.local/servers')
    const { onPresent: onPresentAddServerModal, onDismiss: onDismissAddServerModal } = useModal('add server')
    const { onPresent: onPresentImportServerModal, onDismiss: onDismissImportServerModal } = useModal('import server')
    const theme = useTheme()

    const onClickAddServer = useCallback(() => {
        onPresentAddServerModal()
    }, [onPresentAddServerModal])

    const onClickImportServer = useCallback(() => {
        onPresentImportServerModal()
    }, [onPresentImportServerModal])

    let knownServers = ((knownServersTree || {}).value || []).filter(x => !!x)

    return (
        <Container className={className} verticalPadding={verticalPadding}>
            {knownServers.map(server => (
                <ServerIconWrapper key={server} selected={server === selectedServer} onClick={() => setSelectedServer(server)}>
                    <SFab text={server}>{server.slice(0, 1)}</SFab>
                </ServerIconWrapper>
            ))}

            <Spacer />

            <ServerIconWrapper onClick={onClickImportServer}>
                <CircularButton><ImportIcon style={{ color: theme.color.green[500] }} /></CircularButton>
            </ServerIconWrapper>

            <ServerIconWrapper onClick={onClickAddServer}>
                <CircularButton><AddIcon style={{ color: theme.color.green[500] }} /></CircularButton>
            </ServerIconWrapper>

            <AddServerModal onDismiss={onDismissAddServerModal} />
            <ImportServerModal onDismiss={onDismissImportServerModal} />
        </Container>
    )
}

function AddServerModal({ onDismiss }) {
    const [serverName, setServerName] = useState('')
    const api = useAPI()
    const knownServersTree = useStateTree('chat.local/servers')
    const theme = useTheme()

    let knownServers = (knownServersTree || {}).value || []

    const onClickAdd = useCallback(async () => {
        if (!api) { return }
        try {
            await api.addServer(serverName, knownServers)
            onDismiss()
        } catch (err) {
            console.error(err)
        }
    }, [api, serverName, knownServers, onDismiss])

    function onChangeServerName(e) {
        if (e.code === 'Enter') {
            onClickAdd()
        } else {
            setServerName(e.target.value)
        }
    }

    return (
        <Modal modalKey="add server">
            <ModalTitle>Add a server</ModalTitle>
            <ModalContent>
                <Input value={serverName} onChange={onChangeServerName} />
            </ModalContent>
            <ModalActions>
                <Button onClick={onClickAdd} primary>Add</Button>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

function ImportServerModal({ onDismiss }) {
    const [serverName, setServerName] = useState('')
    const api = useAPI()
    const knownServersTree = useStateTree('chat.local/servers')

    let knownServers = (knownServersTree || {}).value || []

    const onClickImport = useCallback(async () => {
        if (!api) { return }
        try {
            await api.importServer(serverName, knownServers)
            onDismiss()
        } catch (err) {
            console.error(err)
        }
    }, [api, serverName, knownServers])

    function onChangeServerName(e) {
        if (e.code === 'Enter') {
            onClickAdd()
        } else {
            setServerName(e.target.value)
        }
    }

    return (
        <Modal modalKey="import server">
            <ModalTitle>Import a server</ModalTitle>
            <ModalContent>
                <Input value={serverName} onChange={onChangeServerName} />
            </ModalContent>
            <ModalActions>
                <Button onClick={onClickImport} primary>Import</Button>
                <Button onClick={onDismiss}>Cancel</Button>
            </ModalActions>
        </Modal>
    )
}

export default ServerBar