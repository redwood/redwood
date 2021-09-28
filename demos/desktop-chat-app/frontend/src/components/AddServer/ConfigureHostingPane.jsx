import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled from 'styled-components'
import filesize from 'filesize.js'

import Input, { InputLabel } from '../Input'
import Button from '../Button'
import { Pane, PaneContent, PaneActions } from '../SlidingPane'
import Select from '../Select'

import useCreateCloudStackOptions from '../../hooks/useCreateCloudStackOptions'

const SInput = styled(Input)`
    && {
        margin: 0 0 10px;
    }
`

const SButton = styled(Button)`
    && {
        margin: 0 0 10px;
    }
`

const SSelect = styled(Select)`
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

const InstanceStatValue = styled.div``

function ConfigureHostingPane({
    setRequestValues,
    onClickBack,
    onClickNext,
    ...props
}) {
    const inputAPIKeyRef = useRef()
    const [apiKey, setAPIKey] = useState()

    const cloudStackOpts = useCreateCloudStackOptions('linode', apiKey)

    const [sshKey, setSSHKey] = useState()
    const onChangeSSHKey = useCallback(
        (event) => {
            setSSHKey(event.target.value)
        },
        [setSSHKey],
    )

    const [region, setRegion] = useState()
    const onChangeRegion = useCallback(
        (event) => {
            setRegion(event.target.value)
        },
        [setRegion],
    )

    const [instanceType, setInstanceType] = useState()
    const onChangeInstanceType = useCallback(
        (event) => {
            setInstanceType(event.target.value)
        },
        [setInstanceType],
    )
    const chosenInstanceType = cloudStackOpts.instanceTypesMap[instanceType]

    const [image, setImage] = useState()
    const onChangeImage = useCallback(
        (event) => {
            setImage(event.target.value)
        },
        [setImage],
    )

    const [email, setEmail] = useState()
    const onChangeEmail = useCallback(
        (event) => {
            setEmail(event.target.value)
        },
        [setEmail],
    )

    const [password, setPassword] = useState()
    const onChangePassword = useCallback(
        (event) => {
            setPassword(event.target.value)
        },
        [setPassword],
    )

    const [label, setLabel] = useState()
    const onChangeLabel = useCallback(
        (event) => {
            setLabel(event.target.value)
        },
        [setLabel],
    )

    const handleBack = useCallback(() => {
        setRequestValues((prev) => ({
            ...prev,
            apiKey,
            sshKey,
            region,
            instanceType,
            image,
            password,
            label,
            email,
        }))
        onClickBack()
    }, [
        setRequestValues,
        onClickBack,
        apiKey,
        sshKey,
        region,
        instanceType,
        image,
        password,
        label,
        email,
    ])

    const handleNext = useCallback(() => {
        setRequestValues((prev) => ({
            ...prev,
            apiKey,
            sshKey,
            region,
            instanceType,
            image,
            password,
            label,
            email,
        }))
        onClickNext()
    }, [
        setRequestValues,
        onClickNext,
        apiKey,
        sshKey,
        region,
        instanceType,
        image,
        password,
        label,
        email,
    ])

    const onClickSetAPIKey = () => {
        if (!inputAPIKeyRef || !inputAPIKeyRef.current) {
            return
        }
        setAPIKey(inputAPIKeyRef.current.value)
    }

    return (
        <Pane {...props}>
            <PaneContent>
                <InputWithButton>
                    <InputLabel label="API key">
                        <SInput width="100%" ref={inputAPIKeyRef} />
                    </InputLabel>
                    <SButton primary onClick={onClickSetAPIKey}>
                        Set API key
                    </SButton>
                </InputWithButton>

                {apiKey && (
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
                            items={cloudStackOpts.sshKeys.map((x) => ({
                                value: x.label,
                                text: x.label,
                            }))}
                        />
                        <SSelect
                            label="Region"
                            value={region}
                            onChange={onChangeRegion}
                            items={cloudStackOpts.regions.map((x) => ({
                                value: x.id,
                                text: x.id,
                            }))}
                        />
                        <SSelect
                            label="Image"
                            value={image}
                            onChange={onChangeImage}
                            items={cloudStackOpts.images.map((x) => ({
                                value: x.id,
                                text: `${x.label} (${x.size}MB required)`,
                            }))}
                        />

                        <div>
                            <SSelect
                                label="Instance type"
                                value={instanceType}
                                onChange={onChangeInstanceType}
                                items={cloudStackOpts.instanceTypes.map(
                                    (x) => ({
                                        value: x.id,
                                        text: `${x.id} ($${
                                            cloudStackOpts.instanceTypesMap[
                                                x.id
                                            ].monthlyPrice
                                        } / month)`,
                                    }),
                                )}
                            />
                            {chosenInstanceType && (
                                <InstanceStats>
                                    <InstanceStat>
                                        <InstanceStatLabel>
                                            CPUs:
                                        </InstanceStatLabel>
                                        <InstanceStatValue>
                                            {chosenInstanceType.numCPUs}
                                        </InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>
                                            Disk:
                                        </InstanceStatLabel>
                                        <InstanceStatValue>
                                            {filesize(
                                                chosenInstanceType.diskSpaceMB *
                                                    1024 *
                                                    1024,
                                            )}
                                        </InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>
                                            Memory:
                                        </InstanceStatLabel>
                                        <InstanceStatValue>
                                            {filesize(
                                                chosenInstanceType.memoryMB *
                                                    1024 *
                                                    1024,
                                            )}
                                        </InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>
                                            Hourly:
                                        </InstanceStatLabel>
                                        <InstanceStatValue>
                                            ${chosenInstanceType.hourlyPrice}
                                        </InstanceStatValue>
                                    </InstanceStat>
                                    <InstanceStat>
                                        <InstanceStatLabel>
                                            Monthly:
                                        </InstanceStatLabel>
                                        <InstanceStatValue>
                                            ${chosenInstanceType.monthlyPrice}
                                        </InstanceStatValue>
                                    </InstanceStat>
                                </InstanceStats>
                            )}
                        </div>
                    </div>
                )}
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button
                    onClick={handleNext}
                    primary
                    disabled={
                        !sshKey || !region || !image || !chosenInstanceType
                    }
                >
                    Next
                </Button>
            </PaneActions>
        </Pane>
    )
}

export default ConfigureHostingPane
