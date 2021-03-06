import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import filesize from 'filesize.js'
import * as RedwoodReact from 'redwood.js/dist/module/react'

import UploadAvatar from '../UploadAvatar'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input, { InputLabel } from '../Input'
import Stepper from '../Stepper'
import SlidingPane, { Pane, PaneContent, PaneActions } from '../SlidingPane'
import Select from '../Select'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'
import useCreateCloudStackOptions from '../../hooks/useCreateCloudStackOptions'
import theme from '../../theme'
import linodeLogo from '../../assets/linode.png'

const { useStateTree } = RedwoodReact

const SSelect = styled(Select)`
    && {
        margin: 0 0 10px;
    }
`

const SButton = styled(Button)`
    && {
        margin: 0 0 10px;
    }
`

const SInput = styled(Input)`
    && {
        margin: 0 0 10px;
    }
`

const InputWithButton = styled.div`
    display: flex;

    && button {
        flex-shrink: 0;
        height: 30px;
        margin-left: 6px;
        align-self: flex-end;
    }
`

const InstanceStats = styled.div`
    display: flex;
    flex-direction: column;
    width: 150px;
`

const InstanceStat = styled.div`
    display: flex;
    justify-content: space-between;
`

const InstanceStatLabel = styled.div`
    font-weight: 700;
`

const InstanceStatValue = styled.div`
`

const PaneTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 700;
    text-align: center;
    margin-bottom: 12px;
`

const PaneSubtitle = styled.div`
    // font-size: 1.4rem;
    // font-weight: 700;
    // text-align: center;
    margin-bottom: 12px;
`


function AddServerModal({ onDismiss }) {
    const api = useAPI()
    const theme = useTheme()
    let [activeStep, setActiveStep] = useState(0)
    let [requestValues, setRequestValues] = useState({})
    let [provider, setProvider] = useState()

    let steps = !!provider && provider !== 'none'
                    ? ['Name and icon', 'Hosting', 'Configure hosting', 'Confirmation']
                    : ['Name and icon', 'Hosting', 'Confirmation']

    function closeModal() {
        onDismiss()
        setActiveStep(0)
    }

    function onClickNext() {
        if (activeStep === steps.length - 1) { return }
        setActiveStep(activeStep + 1)
    }

    function onClickBack() {
        if (activeStep === 0) { return }
        setActiveStep(activeStep - 1)
    }

    let panes = [{
        title: 'Name and icon',
        width: 500,
        height: 190,
        content: <NameAndIconPane key="one" setRequestValues={setRequestValues} onClickBack={onClickBack} onClickNext={onClickNext} />,
    }, {
        title: 'Hosting',
        width: 640,
        height: 390,
        content: <ChooseHostingPane key="two" provider={provider} setProvider={setProvider} onClickBack={onClickBack} onClickNext={onClickNext} />,
    }, {
        title: 'Configure hosting',
        width: 600,
        height: 690,
        content: <ConfigureHostingPane key="three" setRequestValues={setRequestValues} onClickBack={onClickBack} onClickNext={onClickNext} />,
    }, {
        title: 'Confirmation',
        width: 600,
        height: 510,
        content: <ConfirmationPane key="four" provider={provider} requestValues={requestValues} onClickBack={onClickBack} closeModal={closeModal} />,
    }].filter(p => steps.includes(p.title))

    return (
        <Modal modalKey="add server" closeModal={closeModal}>
            <ModalTitle closeModal={closeModal}>Create a Server</ModalTitle>
            <ModalContent>
                <Stepper activeStep={activeStep} steps={steps} />
                <SlidingPane activePane={activeStep} panes={panes} />
            </ModalContent>
        </Modal>
    )
}

function NameAndIconPane({ setRequestValues, onClickBack, onClickNext, ...props }) {
    const [serverName, setServerName] = useState('')
    const [iconImg, setIconImg] = useState(null)
    const [iconFile, setIconFile] = useState(null)

    const onChangeServerName = useCallback(e => {
        setServerName(e.target.value)
    }, [setServerName])

    const handleBack = useCallback(() => {
        setRequestValues(prev => ({ ...prev, serverName, iconImg, iconFile }))
        onClickBack()
    }, [setRequestValues, onClickBack, serverName, iconImg, iconFile])

    const handleNext = useCallback(() => {
        setRequestValues(prev => ({ ...prev, serverName, iconImg, iconFile }))
        onClickNext()
    }, [setRequestValues, onClickNext, serverName, iconImg, iconFile])

    return (
        <Pane {...props}>
            <PaneContent>
                <UploadAvatar
                    iconImg={iconImg}
                    setIconImg={setIconImg}
                    setIconFile={setIconFile}
                />
                <Input
                    value={serverName}
                    onChange={onChangeServerName}
                    label={'Server Name'}
                    width={'100%'}
                />
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button onClick={handleNext} primary disabled={(serverName || '').trim().length === 0}>Next</Button>
            </PaneActions>
        </Pane>
    )
}

const HostingProviderContainer = styled.div`
    display: flex;
    justify-content: space-evenly;
`

const HostingProviderBox = styled.div`
    width: 150px;
    height: 150px;
    padding: 20px;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    border-radius: 12px;
    cursor: pointer;
    border: ${props => !!props.highlighted ? `4px solid ${theme.color.indigo[500]}` : 'unset'};
    background-color: ${props => theme.color.grey[200]};
    transition: background-color 0.1s ease-in-out;

    &:hover {
        background-color: ${props => theme.color.grey[100]};

        a {
            color: ${theme.color.white};
        }
    }
`

const ProviderLogo = styled.img`
    width: 100%;
`

const ProviderLink = styled.a`
    color: ${theme.color.indigo[500]};
    text-decoration: none;
    transition: all 0.1s ease-in-out;
    &:visited {
        color: ${theme.color.indigo[500]};
    }
    &:hover {
        text-decoration: underline;
    }
    &:active {
        color: ${theme.color.indigo[500]};
    }
`

function ChooseHostingPane({ provider, setProvider, onClickBack, onClickNext, ...props }) {
    const onClickNone = useCallback(() => {
        setProvider('none')
    }, [setProvider])

    const onClickLinode = useCallback(() => {
        setProvider('linode')
    }, [setProvider])

    const handleBack = useCallback(() => {
        setProvider('none')
        onClickBack()
    }, [onClickBack])

    return (
        <Pane {...props}>
            <PaneContent>
                <PaneTitle>Do you need help hosting a public node?</PaneTitle>
                <PaneSubtitle>
                    Even though Redwood is fully peer-to-peer, it's usually necessary to host a node on a publicly-accessible cloud so that other users can find you. Advanced users who are familiar with setting up infrastructure can skip this step.
                </PaneSubtitle>

                <HostingProviderContainer>
                    <HostingProviderBox onClick={onClickNone} highlighted={!provider || provider === 'none'}>
                        I don't need hosting
                    </HostingProviderBox>

                    <HostingProviderBox onClick={onClickLinode} highlighted={provider === 'linode'}>
                        <ProviderLogo src={linodeLogo} />
                        <ProviderLink href="https://linode.com" target="_blank">linode.com</ProviderLink>
                    </HostingProviderBox>
                </HostingProviderContainer>
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button onClick={onClickNext} primary disabled={(provider || '').trim().length === 0}>Next</Button>
            </PaneActions>
        </Pane>
    )
}

function ConfigureHostingPane({ setRequestValues, onClickBack, onClickNext, ...props }) {
    const inputAPIKeyRef = useRef()
    const [apiKey, setAPIKey] = useState()
    function onClickSetAPIKey(event) {
        if (!inputAPIKeyRef || !inputAPIKeyRef.current) { return }
        setAPIKey(inputAPIKeyRef.current.value)
    }

    const cloudStackOpts = useCreateCloudStackOptions('linode', apiKey)

    const [sshKey, setSSHKey] = useState()
    const onChangeSSHKey = useCallback(event => {
        setSSHKey(event.target.value)
    }, [setSSHKey])

    const [region, setRegion] = useState()
    const onChangeRegion = useCallback(event => {
        setRegion(event.target.value)
    }, [setRegion])

    const [instanceType, setInstanceType] = useState()
    const onChangeInstanceType = useCallback(event => {
        setInstanceType(event.target.value)
    }, [setInstanceType])
    let chosenInstanceType = cloudStackOpts.instanceTypesMap[instanceType]

    const [image, setImage] = useState()
    const onChangeImage = useCallback(event => {
        setImage(event.target.value)
    }, [setImage])

    const [email, setEmail] = useState()
    const onChangeEmail = useCallback(event => {
        setEmail(event.target.value)
    }, [setEmail])

    const [password, setPassword] = useState()
    const onChangePassword = useCallback(event => {
        setPassword(event.target.value)
    }, [setPassword])

    const [label, setLabel] = useState()
    const onChangeLabel = useCallback(event => {
        setLabel(event.target.value)
    }, [setLabel])

    const handleBack = useCallback(() => {
        setRequestValues(prev => ({ ...prev, apiKey, sshKey, region, instanceType, image, password, label, email }))
        onClickBack()
    }, [setRequestValues, onClickBack, apiKey, sshKey, region, instanceType, image, password, label, email])

    const handleNext = useCallback(() => {
        setRequestValues(prev => ({ ...prev, apiKey, sshKey, region, instanceType, image, password, label, email }))
        onClickNext()
    }, [setRequestValues, onClickNext, apiKey, sshKey, region, instanceType, image, password, label, email])

    return (
        <Pane {...props}>
            <PaneContent>
                <InputWithButton>
                    <InputLabel label="API key">
                        <SInput
                            width="100%"
                            ref={inputAPIKeyRef}
                        />
                    </InputLabel>
                    <SButton primary onClick={onClickSetAPIKey}>Set API key</SButton>
                </InputWithButton>

                {apiKey &&
                    <div>
                        <InputLabel label="Email to associate with domain name">
                            <SInput
                                width="100%"
                                value={email}
                                onChange={onChangeEmail}
                            />
                        </InputLabel>

                        <InputLabel label="Server password">
                            <SInput
                                type="password"
                                width="100%"
                                value={password}
                                onChange={onChangePassword}
                            />
                        </InputLabel>

                        <InputLabel label="Resource label (optional)">
                            <SInput
                                width="100%"
                                value={label}
                                onChange={onChangeLabel}
                            />
                        </InputLabel>

                        <SSelect
                            label="SSH key"
                            value={sshKey}
                            onChange={onChangeSSHKey}
                            items={cloudStackOpts.sshKeys.map(x => ({ value: x.label, text: x.label }))}
                        />
                        <SSelect
                            label="Region"
                            value={region}
                            onChange={onChangeRegion}
                            items={cloudStackOpts.regions.map(x => ({ value: x.id, text: x.id }))}
                        />
                        <SSelect
                            label="Image"
                            value={image}
                            onChange={onChangeImage}
                            items={cloudStackOpts.images.map(x => ({ value: x.id, text: `${x.label} (${x.size}MB required)` }))}
                        />

                        <div>
                            <SSelect
                                label="Instance type"
                                value={instanceType}
                                onChange={onChangeInstanceType}
                                items={cloudStackOpts.instanceTypes.map(x => ({ value: x.id, text: `${x.id} ($${cloudStackOpts.instanceTypesMap[x.id].monthlyPrice} / month)` }))}
                            />
                            {chosenInstanceType &&
                                <InstanceStats>
                                    <InstanceStat>
                                        <InstanceStatLabel>CPUs:</InstanceStatLabel>
                                        <InstanceStatValue>{chosenInstanceType.numCPUs}</InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>Disk:</InstanceStatLabel>
                                        <InstanceStatValue>{filesize(chosenInstanceType.diskSpaceMB * 1024 * 1024)}</InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>Memory:</InstanceStatLabel>
                                        <InstanceStatValue>{filesize(chosenInstanceType.memoryMB * 1024 * 1024)}</InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>Hourly:</InstanceStatLabel>
                                        <InstanceStatValue>${chosenInstanceType.hourlyPrice}</InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>Monthly:</InstanceStatLabel>
                                        <InstanceStatValue>${chosenInstanceType.monthlyPrice}</InstanceStatValue>
                                    </InstanceStat>
                                </InstanceStats>
                            }
                        </div>
                    </div>
                }
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button onClick={handleNext} primary disabled={!sshKey || !region || !image || !chosenInstanceType}>Next</Button>
            </PaneActions>
        </Pane>
    )
}

function ConfirmationPane({ provider, requestValues, onClickBack, closeModal, ...props }) {
    const api = useAPI()
    const { navigate } = useNavigation()
    const knownServersTree = useStateTree('chat.local/servers')
    let knownServers = (knownServersTree || {}).value || []

    const onClickAdd = useCallback(async () => {
        if (!api) { return }
        try {
            let { apiKey, iconFile, iconImg, image, instanceType, region, serverName, sshKey, label, password, email } = requestValues
            await api.addServer(serverName, knownServers, iconFile, provider, {
                apiKey,
                domainName: serverName,
                domainEmail: email,
                instanceLabel: label,
                instanceRegion: region,
                instancePassword: password,
                instanceType: instanceType,
                instanceImage: image,
                instanceSSHKey: sshKey,
            })
            closeModal()
            navigate(serverName, null)
        } catch (err) {
            console.error(err)
        }
    }, [api, requestValues, knownServers, closeModal, requestValues, navigate])

    return (
        <Pane {...props}>
            <PaneContent>
                <PaneTitle>Confirmation</PaneTitle>

            </PaneContent>

            <PaneActions>
                <Button onClick={onClickBack}>Back</Button>
                <Button onClick={onClickAdd} primary disabled={(provider || '').trim().length === 0}>Add server</Button>
            </PaneActions>
        </Pane>
    )
}

export default AddServerModal