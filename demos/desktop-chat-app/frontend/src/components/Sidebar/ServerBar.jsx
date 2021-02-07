import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, Fab, IconButton, TextField } from '@material-ui/core'
import { Add as AddIcon, CloudDownloadRounded as ImportIcon } from '@material-ui/icons'

import Tooltip from '../Tooltip'
import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import AddServerModal from './AddServerModal'
import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import strToColor from '../../utils/strToColor'
import theme from '../../theme'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    padding: ${props => props.verticalPadding} 0;

    overflow-y: scroll;
    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
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
    transition: .12s ease-in-out all !important;
    background-color: ${props => strToColor(props.text)} !important;
    color: ${props => props.theme.color.white} !important;
    font-weight: 700 !important;
    font-size: 1.1rem !important;
    overflow: hidden;

    img {
      height: 50px;
      border-radius: 100%;
    }

    &:hover {
        // border-radius: 20px !important;
        transform: scale(1.1);
    }
`

const PrimaryButton = styled(Button)`
    background-color: ${props => props.theme.color.green} !important;
`

function ServerFab({ serverName }) {
    const stateTree = useStateTree(`${serverName}/registry`)

    if (stateTree && stateTree.iconImg) {
        return (
            <SFab text={serverName}>
                <img src={`http://localhost:8080/iconImg?state_uri=${serverName}/registry`} />
            </SFab>
        )
    }
    return <SFab text={serverName}>{serverName.slice(0, 1)}</SFab>
}

function ServerBar({ className, verticalPadding }) {
    const { selectedServer, navigate } = useNavigation()
    const [isLoaded, setIsLoaded] = useState(false)
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

    useEffect(() => {
        if (!isLoaded && knownServers.length > 0) {
            setIsLoaded(true)
            navigate(knownServers[0], null)
        }
    }, [knownServersTree])

    return (
        <Container className={className} verticalPadding={verticalPadding}>
            {knownServers.map(server => (
                <Tooltip title={server} placement="right" arrow key={server}>
                    <ServerIconWrapper selected={server === selectedServer} onClick={() => navigate(server, null)}>
                      <ServerFab serverName={server} />
                    </ServerIconWrapper>
                </Tooltip>
            ))}

            <Spacer />

            <Tooltip title="Import existing server" placement="right" arrow>
                <ServerIconWrapper onClick={onClickImportServer}>
                    <CircularButton><ImportIcon style={{ color: theme.color.indigo[500] }} /></CircularButton>
                </ServerIconWrapper>
            </Tooltip>

            <Tooltip title="Create new server" placement="right" arrow>
                <ServerIconWrapper onClick={onClickAddServer}>
                    <CircularButton><AddIcon style={{ color: theme.color.indigo[500] }} /></CircularButton>
                </ServerIconWrapper>
            </Tooltip>

            <AddServerModal onDismiss={onDismissAddServerModal} />
            <ImportServerModal onDismiss={onDismissImportServerModal} />
        </Container>
    )
}



const SInput = styled(Input)`
    width: 460px;
`

function ImportServerModal({ onDismiss }) {
    const { navigate } = useNavigation()
    const [serverName, setServerName] = useState('')
    const api = useAPI()
    const knownServersTree = useStateTree('chat.local/servers')

    let knownServers = (knownServersTree || {}).value || []

    const onClickImport = useCallback(async () => {
        if (!api) { return }
        try {
            await api.importServer(serverName, knownServers)
            onDismiss()
            navigate(serverName, null)
        } catch (err) {
            console.error(err)
        }
    }, [api, serverName, knownServers])

    function onChangeServerName(e) {
        if (e.code === 'Enter') {
            onClickImport()
        } else {
            setServerName(e.target.value)
        }
    }

    return (
        <Modal modalKey="import server">
            <ModalTitle closeModal={onDismiss}>Import a Server</ModalTitle>
            <ModalContent>
                <SInput
                    value={serverName}
                    onChange={onChangeServerName}
                    label={'Server Name'}
                    width={'460px'}
                />
            </ModalContent>
            <ModalActions>
                <Button onClick={onClickImport} primary>Import</Button>
            </ModalActions>
        </Modal>
    )
}

export default ServerBar