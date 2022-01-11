import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar, Fab, IconButton, TextField } from '@material-ui/core'
import { Add as AddIcon, CloudDownloadRounded as ImportIcon, Face as FaceIcon } from '@material-ui/icons'
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
import { useRedwood, useStateTree } from '@redwood.dev/client/react'
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

const SStaticRelayFormWrapper = styled.div`
    display: flex;
    flex-direction: row;
`

const SSettingsSectionHeader = styled.h4`
    margin-top: 8px;
    margin-bottom: 8px;
`

const SAddStaticRelayInput = styled(Input)`
    margin-right: 16px;
`

const SNoStaticRelaysText = styled.div`
    color: ${props => props.theme.color.grey[100]};
    margin-bottom: 16px;
`

const SStaticRelaysContainer = styled.ul`
    margin-bottom: 28px;
`

function SettingsModal({ onDismiss }) {
    const { redwoodClient } = useRedwood()
    const [staticRelays, setStaticRelays] = useState([])
    const [errorMsg, setErrorMsg] = useState(null)
    const newStaticRelayRef = useRef()

    useEffect(() => {
        if (!redwoodClient) {
            return
        }
        (async function() {
            try {
                setStaticRelays(await redwoodClient.rpc.staticRelays())
            } catch (err) {
                setErrorMsg(err.toString())
            }
        })()
    }, [redwoodClient, setStaticRelays, setErrorMsg])

    const onClickAddStaticRelay = useCallback(async () => {
        if (!redwoodClient) {
            return
        }
        try {
            await redwoodClient.rpc.addStaticRelay(newStaticRelayRef.current.value)
            setStaticRelays(await redwoodClient.rpc.staticRelays())
            newStaticRelayRef.current.value = ''
        } catch (err) {
            setErrorMsg(err.toString())
        }
    }, [redwoodClient, setStaticRelays, setErrorMsg])

    return (
        <Modal modalKey="settings">
            <ModalTitle closeModal={onDismiss}>Settings</ModalTitle>
            <SSettingsModalContent>
                {errorMsg &&
                    <SError>{errorMsg}</SError>
                }

                <SSettingsSectionHeader>Static relays</SSettingsSectionHeader>

                {(staticRelays || []).length === 0 &&
                    <SNoStaticRelaysText>No static relays configured.</SNoStaticRelaysText>
                }
                {(staticRelays || []).length > 0 &&
                    <SStaticRelaysContainer>
                        {(staticRelays || []).map(relayAddr => (
                            <li>{relayAddr}</li>
                        ))}
                    </SStaticRelaysContainer>
                }

                <SStaticRelayFormWrapper>
                    <SAddStaticRelayInput ref={newStaticRelayRef} />
                    <Button primary onClick={onClickAddStaticRelay}>Add</Button>
                </SStaticRelayFormWrapper>

            </SSettingsModalContent>
            <ModalActions>
            </ModalActions>
        </Modal>
    )
}

export default SettingsModal