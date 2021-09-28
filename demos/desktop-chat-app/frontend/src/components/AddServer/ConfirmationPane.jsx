import React, { useCallback } from 'react'
import styled from 'styled-components'

import Button from '../Button'
import { Pane, PaneContent, PaneActions } from '../SlidingPane'

import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'

const PaneTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 700;
    text-align: center;
    margin-bottom: 12px;
`

function ConfirmationPane({
    provider,
    requestValues,
    onClickBack,
    closeModal,
    ...props
}) {
    const api = useAPI()
    const { navigate } = useNavigation()

    const onClickAdd = useCallback(async () => {
        if (!api) {
            return
        }
        try {
            const {
                apiKey,
                iconFile,
                image,
                instanceType,
                region,
                serverName,
                sshKey,
                label,
                password,
                email,
            } = requestValues
            await api.addServer(serverName, iconFile, provider, {
                apiKey,
                domainName: serverName,
                domainEmail: email,
                instanceLabel: label,
                instanceRegion: region,
                instancePassword: password,
                instanceType,
                instanceImage: image,
                instanceSSHKey: sshKey,
            })
            closeModal()
            navigate(serverName, null)
        } catch (err) {
            // NOTE: Add error catching here
            throw new Error(err)
        }
    }, [api, requestValues, closeModal, provider, navigate])

    return (
        <Pane {...props}>
            <PaneContent>
                <PaneTitle>Confirmation</PaneTitle>
            </PaneContent>

            <PaneActions>
                <Button onClick={onClickBack}>Back</Button>
                <Button
                    onClick={onClickAdd}
                    primary
                    disabled={(provider || '').trim().length === 0}
                >
                    Add server
                </Button>
            </PaneActions>
        </Pane>
    )
}

export default ConfirmationPane
