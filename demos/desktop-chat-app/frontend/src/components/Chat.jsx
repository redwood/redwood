import React, { useState, useCallback } from 'react'
import styled from 'styled-components'
import { IconButton } from '@material-ui/core'
import { SendRounded as SendIcon } from '@material-ui/icons'
import rpcFetch from '../utils/rpcFetch'
import Sidebar from './Sidebar'
import Button from './Button'
import Input from './Input'
import useAPI from '../hooks/useAPI'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: ${props => props.theme.color.grey[200]};
`

const MessageContainer = styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;

    overflow-y: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
`

const Message = styled.div`
    display: flex;
    flex-direction: column;
    padding: 10px 0;
`

const MessageSender = styled.div`
    font-weight: 700;
`

const MessageText = styled.div`
`

const ControlsContainer = styled.div`
    display: flex;
    align-self: end;
    padding-bottom: 6px;
    width: 100%;
`

const SIconButton = styled(IconButton)`
    color: ${props => props.theme.color.white} !important;
    padding: 0 12px !important;
`

function Chat({ stateURI, className }) {
    const { nodeAddress } = useRedwood()
    const api = useAPI()
    const roomState = useStateTree(stateURI)
    const [messageText, setMessageText] = useState('')

    let server, room
    if (!!stateURI) {
        ([server, room] = stateURI.split('/'))
    }
    let messages = (roomState || {}).messages || []

    const onClickSend = useCallback(async () => {
        if (!api) { return }
        await api.sendMessage(messageText, nodeAddress, server, room, messages)
    }, [messageText, nodeAddress, server, room, messages, api])

    function onChangeMessageText(e) {
        setMessageText(e.target.value)
    }

    function onKeyDown(e) {
        console.log(e)
        if (e.code === 'Enter') {
            onClickSend()
            setMessageText('')
        }
    }

    if (!stateURI) {
        return <Container className={className}></Container>
    }
    return (
        <Container className={className}>
            <MessageContainer>
                {messages.map(msg => (
                    <Message>
                        <MessageSender>{msg.sender}:</MessageSender>
                        <MessageText>{msg.text}</MessageText>
                    </Message>
                ))}
            </MessageContainer>

            <ControlsContainer>
                <Input onKeyDown={onKeyDown} onChange={onChangeMessageText} value={messageText} />
                <SIconButton onClick={onClickSend}><SendIcon /></SIconButton>
            </ControlsContainer>
        </Container>
    )
}

export default Chat
