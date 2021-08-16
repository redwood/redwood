import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, IconButton, TextField } from '@material-ui/core'
import { Add as AddIcon, CloudDownloadRounded as ImportIcon, Face as FaceIcon } from '@material-ui/icons'
import moment from 'moment'

import Tooltip from '../Tooltip'
import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
import { ServerFab, DMButton } from '../ServerFab'
import AddServerModal from './AddServerModal'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useAddressBook from '../../hooks/useAddressBook'
import useJoinedServers from '../../hooks/useJoinedServers'
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

const CircularButton = styled(IconButton)`
    border-radius: 9999;
    background-color: ${props => props.theme.color.grey[200]} !important;
`

const Spacer = styled.div`
    flex-grow: 1;
`

function ServerBar({ className, verticalPadding }) {
    const { selectedServer, navigate } = useNavigation()
    const [isLoaded, setIsLoaded] = useState(false)
    const joinedServers = useJoinedServers()

    const { onPresent: onPresentContactsModal } = useModal('contacts')
    const { onPresent: onPresentAddServerModal, onDismiss: onDismissAddServerModal } = useModal('add server')
    const { onPresent: onPresentImportServerModal, onDismiss: onDismissImportServerModal } = useModal('import server')
    const theme = useTheme()

    const onClickContacts = useCallback(() => {
        onPresentContactsModal()
    }, [onPresentContactsModal])

    const onClickAddServer = useCallback(() => {
        onPresentAddServerModal()
    }, [onPresentAddServerModal])

    const onClickImportServer = useCallback(() => {
        onPresentImportServerModal()
    }, [onPresentImportServerModal])

    useEffect(() => {
        if (!isLoaded && joinedServers.length > 0) {
            setIsLoaded(true)
            navigate(joinedServers[0], null)
        }
    }, [joinedServers])

    return (
        <Container className={className} verticalPadding={verticalPadding}>
            <Tooltip title="Direct messages" placement="right" arrow>
                <ServerIconWrapper selected={selectedServer === 'chat.p2p'}>
                    <ServerFab serverName="chat.p2p" navigateOnClick />
                </ServerIconWrapper>
            </Tooltip>

            {joinedServers.map(server => (
                <Tooltip title={server} placement="right" arrow key={server}>
                    <ServerIconWrapper selected={server === selectedServer}>
                        <ServerFab serverName={server} navigateOnClick />
                    </ServerIconWrapper>
                </Tooltip>
            ))}

            <Spacer />

            <Tooltip title="Contacts" placement="right" arrow>
                <ServerIconWrapper onClick={onClickContacts}>
                    <CircularButton><FaceIcon style={{ color: theme.color.indigo[500] }} /></CircularButton>
                </ServerIconWrapper>
            </Tooltip>

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

    const onClickImport = useCallback(async () => {
        if (!api) { return }
        try {
            await api.importServer(serverName)
            onDismiss()
            navigate(serverName, null)
        } catch (err) {
            console.error(err)
        }
    }, [api, serverName])

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