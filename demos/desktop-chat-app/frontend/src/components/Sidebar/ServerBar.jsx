import React, { useState, useCallback, useEffect, memo } from 'react'
import styled from 'styled-components'
import { IconButton } from '@material-ui/core'
import {
    Add as AddIcon,
    CloudDownloadRounded as ImportIcon,
    Face as FaceIcon,
} from '@material-ui/icons'

import Tooltip from '../Tooltip'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input, { InputLabel } from '../Input'
import { ServerFab } from '../ServerFab'
import AddServerModal from '../AddServer/Modal'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import useJoinedServers from '../../hooks/useJoinedServers'
import mTheme from '../../theme'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    padding: ${(props) => props.verticalPadding} 0;

    overflow-y: scroll;
    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

const ServerIconWrapper = styled.div`
    display: flex;
    justify-content: center;
    align-items: center;
    padding: 8px 0;
    cursor: pointer;
    text-transform: uppercase;
    transition: 0.12s ease-in-out all;

    &:hover {
        img {
            transform: scale(1.125);
        }
    }
`

const CircularButton = styled(IconButton)`
    border-radius: 9999;
    background-color: ${(props) => props.theme.color.grey[200]} !important;
`

const Spacer = styled.div`
    flex-grow: 1;
`

function ServerBar({ className, verticalPadding }) {
    const { selectedServer, navigate } = useNavigation()
    const [isLoaded, setIsLoaded] = useState(false)
    const joinedServers = useJoinedServers()

    const { onPresent: onPresentContactsModal } = useModal('contacts')
    const {
        onPresent: onPresentAddServerModal,
        onDismiss: onDismissAddServerModal,
    } = useModal('add server')
    const {
        onPresent: onPresentImportServerModal,
        onDismiss: onDismissImportServerModal,
    } = useModal('import server')

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
    }, [joinedServers, isLoaded, navigate])

    return (
        <Container className={className} verticalPadding={verticalPadding}>
            <Tooltip title="Direct messages" placement="right" arrow>
                <ServerIconWrapper selected={selectedServer === 'chat.p2p'}>
                    <ServerFab serverName="chat.p2p" navigateOnClick />
                </ServerIconWrapper>
            </Tooltip>

            {joinedServers.map((server) => (
                <Tooltip title={server} placement="right" arrow key={server}>
                    <ServerIconWrapper selected={server === selectedServer}>
                        <ServerFab serverName={server} navigateOnClick />
                    </ServerIconWrapper>
                </Tooltip>
            ))}

            <Spacer />

            <Tooltip title="Contacts" placement="right" arrow>
                <ServerIconWrapper onClick={onClickContacts}>
                    <CircularButton>
                        <FaceIcon style={{ color: mTheme.color.indigo[500] }} />
                    </CircularButton>
                </ServerIconWrapper>
            </Tooltip>

            <Tooltip title="Join existing server" placement="right" arrow>
                <ServerIconWrapper onClick={onClickImportServer}>
                    <CircularButton>
                        <ImportIcon
                            style={{ color: mTheme.color.indigo[500] }}
                        />
                    </CircularButton>
                </ServerIconWrapper>
            </Tooltip>

            <Tooltip title="Create new server" placement="right" arrow>
                <ServerIconWrapper onClick={onClickAddServer}>
                    <CircularButton>
                        <AddIcon style={{ color: mTheme.color.indigo[500] }} />
                    </CircularButton>
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
        if (!api) {
            return
        }
        try {
            await api.importServer(serverName)
            onDismiss()
            navigate(serverName, null)
        } catch (err) {
            // NOTE: Add proper error catching here
            throw new Error(err)
        }
    }, [api, serverName, onDismiss, navigate])

    const onChangeServerName = (e) => {
        if (e.code === 'Enter') {
            onClickImport()
        } else {
            setServerName(e.target.value)
        }
    }

    return (
        <Modal modalKey="import server">
            <ModalTitle closeModal={onDismiss}>Join a Server</ModalTitle>
            <ModalContent>
                <InputLabel label="Server Name">
                    <SInput
                        value={serverName}
                        onChange={onChangeServerName}
                        label="Server Name"
                        width="460px"
                    />
                </InputLabel>
            </ModalContent>
            <ModalActions>
                <Button onClick={onClickImport} primary>
                    Join
                </Button>
            </ModalActions>
        </Modal>
    )
}

export default memo(ServerBar)
