import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, Fab, IconButton, TextField, List, ListItem, ListItemText, ListItemSecondaryAction } from '@material-ui/core'
import { Add as AddIcon, CloudDownloadRounded as ImportIcon, Face as FaceIcon, Delete as DeleteIcon } from '@material-ui/icons'
import moment from 'moment'

import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import SlidingPane, { Pane, PaneContent, PaneActions } from '../SlidingPane'
import UserAvatar from '../UserAvatar'
import Tabs from '../Tabs'
import Select from '../Select'
import Input, { InputLabel } from '../Input'
import { ServerFab } from '../ServerFab'
import PeerRow from '../PeerRow'
import { useRedwood, useStateTree } from '@redwood.dev/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useAddressBook from '../../hooks/useAddressBook'
import strToColor from '../../utils/strToColor'
import theme from '../../theme'

const SSettingsModalContent = styled(ModalContent)`
    max-height: 60vh;
    overflow: scroll;
    min-width: 600px;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
`

const SError = styled.div`
    color: red;
`

const SFormWrapper = styled.div`
    display: flex;
    flex-direction: row;
    padding: 16px;
`

const SSettingsSectionHeader = styled.h4`
    margin-top: 8px;
    margin-bottom: 16px;
`

const SAddStaticRelayInput = styled(Input)`
    margin-right: 16px;
`

const SNoItemsText = styled.div`
    color: ${props => props.theme.color.grey[100]};
    margin-bottom: 16px;
`

const SStaticRelaysContainer = styled.ul`
    margin-bottom: 28px;
`

const Spacer = styled.div`
    height: 24px;
`

const SectionWrapper = styled.div`
    background-color: ${props => props.theme.color.grey[500]};
    border-radius: 8px;
    padding: 4px 0;
    border: 1px solid ${props => props.theme.color.grey[600]};
`

function SettingsModal({ onDismiss }) {
    const { redwoodClient } = useRedwood()
    const [staticRelays, setStaticRelays] = useState([])
    const [vaults, setVaults] = useState([])
    const [errorMsg, setErrorMsg] = useState(null)
    const newStaticRelayRef = useRef()
    const newVaultRef = useRef()

    useEffect(() => {
        if (!redwoodClient) {
            return
        }
        (async function() {
            try {
                setStaticRelays(await redwoodClient.rpc.staticRelays())
                setVaults(await redwoodClient.rpc.vaults())
                setErrorMsg('')
            } catch (err) {
                setErrorMsg(err.toString())
            }
        })()
    }, [redwoodClient, setStaticRelays, setVaults, setErrorMsg])

    const onClickAddStaticRelay = useCallback(async () => {
        if (!redwoodClient) {
            return
        }
        try {
            await redwoodClient.rpc.addStaticRelay(newStaticRelayRef.current.value)
            setStaticRelays(await redwoodClient.rpc.staticRelays())
            newStaticRelayRef.current.value = ''
            setErrorMsg('')
        } catch (err) {
            setErrorMsg(err.toString())
        }
    }, [redwoodClient, setStaticRelays, setErrorMsg])

    const onClickDeleteStaticRelay = useCallback(async (addr) => {
        if (!redwoodClient) {
            return
        }
        try {
            await redwoodClient.rpc.removeStaticRelay(addr)
            setStaticRelays(await redwoodClient.rpc.staticRelays())
            setErrorMsg('')
        } catch (err) {
            setErrorMsg(err.toString())
        }
    }, [redwoodClient, setStaticRelays, setErrorMsg])

    const onClickAddVault = useCallback(async () => {
        if (!redwoodClient) {
            return
        }
        try {
            await redwoodClient.rpc.addVault(newVaultRef.current.value)
            setVaults(await redwoodClient.rpc.vaults())
            newVaultRef.current.value = ''
            setErrorMsg('')
        } catch (err) {
            setErrorMsg(err.toString())
        }
    }, [redwoodClient, setVaults, setErrorMsg])

    const onClickDeleteVault = useCallback(async (addr) => {
        if (!redwoodClient) {
            return
        }
        try {
            await redwoodClient.rpc.removeVault(addr)
            setVaults(await redwoodClient.rpc.vaults())
            setErrorMsg('')
        } catch (err) {
            setErrorMsg(err.toString())
        }
    }, [redwoodClient, setVaults, setErrorMsg])

    return (
        <Modal modalKey="settings">
            <ModalTitle closeModal={onDismiss}>Settings</ModalTitle>
            <SSettingsModalContent>
                {errorMsg &&
                    <SError>{errorMsg}</SError>
                }

                <SSettingsSectionHeader>Vaults</SSettingsSectionHeader>

                <SectionWrapper>
                {(vaults || []).length === 0 &&
                    <SNoItemsText>No vaults configured.</SNoItemsText>
                }
                {(vaults || []).length > 0 &&
                    <List dense>
                        {(vaults || []).map(addr => (
                            <ListItem>
                                <ListItemText primary={addr} />
                                <ListItemSecondaryAction>
                                    <IconButton onClick={() => onClickDeleteVault(addr)} edge="end" aria-label="delete">
                                        <DeleteIcon style={{ color: theme.color.green[500] }} />
                                    </IconButton>
                                </ListItemSecondaryAction>
                            </ListItem>
                        ))}
                    </List>
                }

                <SFormWrapper>
                    <SAddStaticRelayInput onEnter={onClickAddVault} ref={newVaultRef} />
                    <Button primary onClick={onClickAddVault}>Add</Button>
                </SFormWrapper>
                </SectionWrapper>

                <Spacer />

                <SSettingsSectionHeader>Static relays</SSettingsSectionHeader>

                <SectionWrapper>
                {(staticRelays || []).length === 0 &&
                    <SNoItemsText>No static relays configured.</SNoItemsText>
                }
                {(staticRelays || []).length > 0 &&
                    <List dense>
                        {(staticRelays || []).map(addr => (
                            <ListItem>
                                <ListItemText primary={addr} />
                                <ListItemSecondaryAction>
                                    <IconButton onClick={() => onClickDeleteStaticRelay(addr)} edge="end" aria-label="delete">
                                        <DeleteIcon style={{ color: theme.color.green[500] }} />
                                    </IconButton>
                                </ListItemSecondaryAction>
                            </ListItem>
                        ))}
                    </List>
                }

                <SFormWrapper>
                    <SAddStaticRelayInput onEnter={onClickAddStaticRelay} ref={newStaticRelayRef} />
                    <Button primary onClick={onClickAddStaticRelay}>Add</Button>
                </SFormWrapper>
                </SectionWrapper>

            </SSettingsModalContent>
            <ModalActions>
            </ModalActions>
        </Modal>
    )
}

export default SettingsModal