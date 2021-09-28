import React, { useState, useCallback, useEffect } from 'react'
import styled from 'styled-components'

import Modal, { ModalTitle, ModalContent } from '../Modal'
import Button from '../Button'
import SlidingPane, { Pane, PaneContent, PaneActions } from '../SlidingPane'
import Tabs from '../Tabs'
import Select from '../Select'
import Input, { InputLabel } from '../Input'
import { ServerFab } from '../ServerFab'
import PeerRow from '../PeerRow'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import usePeers from '../../hooks/usePeers'

function ContactsModal({ onDismiss }) {
    const {
        activeModalProps: { initiallyFocusedContact },
    } = useModal('contacts')
    const [activeStep, setActiveStep] = useState(1)
    const [selectedPeer, setSelectedPeer] = useState(initiallyFocusedContact)

    useEffect(() => {
        // setActiveStep(initiallyFocusedContact ? 2 : 1)
        setSelectedPeer(initiallyFocusedContact)
    }, [initiallyFocusedContact])

    const showAddPeer = useCallback(() => {
        setActiveStep(0)
    }, [setActiveStep])

    const showPeerList = useCallback(() => {
        setActiveStep(1)
    }, [setActiveStep])

    const showPeerDetails = useCallback(
        (address) => {
            setActiveStep(2)
            setSelectedPeer(address)
        },
        [setActiveStep, setSelectedPeer],
    )

    const onClickBack = useCallback(() => {
        if (activeStep === 0) {
            return
        }
        setActiveStep(activeStep - 1)
    }, [setActiveStep, activeStep])

    const handleDismiss = useCallback(() => {
        setActiveStep(1)
        setSelectedPeer(null)
        onDismiss()
    }, [onDismiss, setActiveStep, setSelectedPeer])

    const onOpen = useCallback(() => {
        if (initiallyFocusedContact) {
            showPeerDetails(initiallyFocusedContact)
        } else {
            showPeerList()
        }
    }, [initiallyFocusedContact, showPeerDetails, showPeerList])

    const panes = [
        {
            width: '480px',
            height: '190px',
            content: <AddPeerPane key="one" showPeerList={showPeerList} />,
        },
        {
            width: '480px',
            height: '50vh',
            content: (
                <PeerListPane
                    key="two"
                    showAddPeer={showAddPeer}
                    showPeerDetails={showPeerDetails}
                />
            ),
        },
        {
            width: '800px',
            height: '390px',
            content: (
                <PeerDetailPane
                    key="three"
                    selectedPeer={selectedPeer}
                    onClickBack={onClickBack}
                />
            ),
        },
    ]

    return (
        <Modal modalKey="contacts" onOpen={onOpen} closeModal={handleDismiss}>
            <ModalTitle closeModal={handleDismiss}>Contacts</ModalTitle>
            <ModalContent>
                <SlidingPane activePane={activeStep} panes={panes} />
            </ModalContent>
        </Modal>
    )
}

const SInputLabel = styled(InputLabel)`
    margin-top: 16px;
`

function AddPeerPane({ showPeerList, ...props }) {
    const api = useAPI()

    const [transport, setTransport] = useState('libp2p')
    const onChangeTransport = useCallback(
        (event) => {
            setTransport(event.target.value)
        },
        [setTransport],
    )

    const [dialAddr, setDialAddr] = useState()

    const onClickSave = useCallback(async () => {
        await api.addPeer(transport, dialAddr)
        showPeerList()
    }, [api, showPeerList, transport, dialAddr])

    const onClickCancel = useCallback(() => {
        setTransport('libp2p')
        setDialAddr('')
        showPeerList()
    }, [showPeerList])

    return (
        <Pane {...props}>
            <PaneContent>
                <Select
                    label="Transport"
                    value={transport}
                    onChange={onChangeTransport}
                    items={[
                        { value: 'libp2p', text: 'libp2p' },
                        { value: 'braidhttp', text: 'http' },
                    ]}
                />
                <SInputLabel label="Dial address">
                    <Input
                        value={dialAddr}
                        onChange={(event) =>
                            setDialAddr(event.currentTarget.value)
                        }
                    />
                </SInputLabel>
            </PaneContent>
            <PaneActions>
                <Button onClick={onClickSave}>Save</Button>
                <Button onClick={onClickCancel}>Cancel</Button>
            </PaneActions>
        </Pane>
    )
}

const SPeerListPaneContent = styled(Pane)`
    overflow-y: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

function PeerListPane({ showAddPeer, showPeerDetails, ...props }) {
    const { peersByAddress } = usePeers()
    const peers = Object.keys(peersByAddress)
        .map((addr) => peersByAddress[addr])
        .filter((peer) => !peer.isSelf)
    return (
        <Pane {...props}>
            <SPeerListPaneContent>
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
                {peers.map((peer) => (
                    <PeerRow
                        address={peer.address}
                        onClick={() => showPeerDetails(peer.address)}
                        key={peer.address}
                    />
                ))}
            </SPeerListPaneContent>
            <PaneActions>
                <Button onClick={showAddPeer}>Add peer</Button>
            </PaneActions>
        </Pane>
    )
}

// const PeerNameContainer = styled.div`
//     display: flex;
// `

// const PeerNameTitle = styled.h3`
//     margin: 0;
//     cursor: pointer;
// `

// const PeerNameSubtitle = styled.h4`
//     margin: 0;
//     color: ${(props) => props.theme.color.grey[100]};
//     font-weight: 300;
// `

// const PeerLastSeen = styled.div`
//     // margin-top: -4px;
//     font-size: 0.8rem;
//     color: ${(props) => props.theme.color.grey[100]};
// `

// const SUserAvatar = styled(UserAvatar)`
//     margin-right: 12px;
// `

const STabs = styled(Tabs)`
    margin-top: 20px;
`

const TransportsView = styled.div`
    background-color: ${(props) => props.theme.color.grey[500]};
    border: 1px solid ${(props) => props.theme.color.grey[600]};
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
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

const SServerFab = styled(ServerFab)`
    && {
        margin-left: 12px;
        margin-right: 12px;
        box-shadow: none;
    }
`

function PeerDetailPane({
    selectedPeer,
    showPeerDetails,
    onClickBack,
    ...props
}) {
    const { peersByAddress } = usePeers()
    const { onDismiss } = useModal('contacts')
    const { navigate } = useNavigation()

    const onClickServer = useCallback(
        (server) => {
            navigate(server, null)
            onDismiss()
        },
        [navigate, onDismiss],
    )

    if (!selectedPeer) {
        return null
    }
    const peer = peersByAddress[selectedPeer]
    if (!peer) {
        return null
    }

    let tabs = Object.keys(peer.transports).map((transport) => ({
        title: transport,
        content: (
            <TransportsView>
                {peer.transports[transport].map((addr) => (
                    <div key={addr}>{addr}</div>
                ))}
            </TransportsView>
        ),
    }))
    tabs = [
        {
            title: 'Servers',
            content: (
                <div>
                    {peer.servers.map((server, idx) => (
                        <div
                            key={server}
                            onClick={() => onClickServer(server)}
                            role="tab"
                            tabIndex={idx}
                        >
                            <SServerFab serverName={server} navigateOnClick />
                            {server}
                        </div>
                    ))}
                </div>
            ),
        },
        ...tabs,
    ]

    return (
        <Pane {...props}>
            <PaneContent>
                <PeerRow
                    address={selectedPeer}
                    editable
                    showLastSeen
                    boldName
                />
                <STabs tabs={tabs} initialActiveTab={0} />
            </PaneContent>

            <PaneActions>
                <Button onClick={onClickBack}>Back</Button>
            </PaneActions>
        </Pane>
    )
}

export default ContactsModal
