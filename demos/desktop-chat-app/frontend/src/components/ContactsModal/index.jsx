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
import { ServerFab } from '../ServerFab'
import PeerRow from '../PeerRow'
import { useRedwood, useStateTree } from 'redwood/dist/main/react'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'
import useAddressBook from '../../hooks/useAddressBook'
import strToColor from '../../utils/strToColor'
import theme from '../../theme'

function ContactsModal({ onDismiss }) {
    let { activeModalProps: { initiallyFocusedContact } } = useModal('contacts')
    let [activeStep, setActiveStep] = useState(initiallyFocusedContact ? 1 : 0)
    let [selectedPeer, setSelectedPeer] = useState(initiallyFocusedContact)

    useEffect(() => {
        setActiveStep(initiallyFocusedContact ? 1 : 0)
        setSelectedPeer(initiallyFocusedContact)
    }, [initiallyFocusedContact])

    let showPeerDetails = useCallback(address => {
        setActiveStep(activeStep + 1)
        setSelectedPeer(address)
    }, [setActiveStep, activeStep])

    let onClickBack = useCallback(() => {
        if (activeStep === 0) { return }
        setActiveStep(activeStep - 1)
    }, [setActiveStep, activeStep])

    let handleDismiss = useCallback(() => {
        setActiveStep(0)
        setSelectedPeer(null)
        onDismiss()
    }, [onDismiss, setActiveStep, setSelectedPeer])

    let panes = [{
        width: 480,
        height: 190,
        content: <PeerListPane key="one" showPeerDetails={showPeerDetails} />,
    }, {
        width: 800,
        height: 390,
        content: <PeerDetailPane key="two" selectedPeer={selectedPeer} onClickBack={onClickBack} />,
    }]

    return (
        <Modal modalKey="contacts">
            <ModalTitle closeModal={handleDismiss}>Contacts</ModalTitle>
            <ModalContent>
                <SlidingPane activePane={activeStep} panes={panes} />
            </ModalContent>
        </Modal>
    )
}

function PeerListPane({ showPeerDetails, ...props }) {
    let { peersByAddress } = usePeers()
    let peers = Object.keys(peersByAddress).map(addr => peersByAddress[addr]).filter(peer => !peer.isSelf)
    return (
        <Pane {...props}>
            <PaneContent>
                {peers.map(peer => (
                    <PeerRow address={peer.address} onClick={() => showPeerDetails(peer.address)} key={peer.address} />
                ))}
            </PaneContent>
        </Pane>
    )
}

const PeerNameContainer = styled.div`
    display: flex;
`

const PeerNameTitle = styled.h3`
    margin: 0;
    cursor: pointer;
`

const PeerNameSubtitle = styled.h4`
    margin: 0;
    color: ${props => props.theme.color.grey[100]};
    font-weight: 300;
`

const PeerLastSeen = styled.div`
    // margin-top: -4px;
    font-size: 0.8rem;
    color: ${props => props.theme.color.grey[100]};
`

const SUserAvatar = styled(UserAvatar)`
    margin-right: 12px;
`

const STabs = styled(Tabs)`
    margin-top: 20px;
`

const TransportsView = styled.div`
    background-color: ${props => props.theme.color.grey[500]};
    border: 1px solid ${props => props.theme.color.grey[600]};
    font-family: Consolas, 'Courier New', monospace;
    font-size: 1rem;
    line-height: 1.3rem;
    padding: 8px;
    border-radius: 4px;

    height: 174px;
    overflow-y: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
`

const SServerFab = styled(ServerFab)`
    && {
        margin-left: 12px;
        margin-right: 12px;
        box-shadow: none;
    }
`

function PeerDetailPane({ selectedPeer, showPeerDetails, onClickBack, ...props }) {
    let { peersByAddress } = usePeers()
    let { onDismiss } = useModal('contacts')
    let { navigate } = useNavigation()

    let onClickServer = useCallback((server) => {
        navigate(server, null)
        onDismiss()
    }, [navigate, onDismiss])

    if (!selectedPeer) {
        return null
    }
    let peer = peersByAddress[selectedPeer]
    if (!peer) {
        return null
    }

    let tabs = Object.keys(peer.transports).map(transport => ({
        title: transport,
        content: (
            <TransportsView>
                {peer.transports[transport].map(addr => (
                    <div key={addr}>{addr}</div>
                ))}
            </TransportsView>
        ),
    }))
    tabs = [ {
        title: 'Servers',
        content: (
            <div>
                {peer.servers.map(server => (
                    <div key={server} onClick={() => onClickServer(server)}>
                        <SServerFab serverName={server} navigateOnClick />
                        {server}
                    </div>
                ))}
            </div>
        ),
    }, ...tabs ]

    return (
        <Pane {...props}>
            <PaneContent>
                <PeerRow address={selectedPeer} editable showLastSeen boldName />
                <STabs tabs={tabs} initialActiveTab={0} />
            </PaneContent>

            <PaneActions>
                <Button onClick={onClickBack}>Back</Button>
            </PaneActions>
        </Pane>
    )
}

export default ContactsModal