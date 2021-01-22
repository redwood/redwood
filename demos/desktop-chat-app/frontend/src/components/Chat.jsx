import React, { useState, useCallback } from 'react'
import styled from 'styled-components'
import rpcFetch from '../utils/rpcFetch'
import Sidebar from './Sidebar'
import Button from './Button'
import Input from './Input'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'
import * as api from '../api'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    height: 100%;
`

const MessageContainer = styled.div`
    display: flex;
    flex-direction: column;
    overflow-y: scroll;
    flex-grow: 1;
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

function Chat({ stateURI, className }) {
    const { nodeAddress } = useRedwood()
    const roomState = useStateTree(stateURI)
    const [messageText, setMessageText] = useState('')

    let server, room
    if (!!stateURI) {
        ([server, room] = stateURI.split('/'))
    }
    let messages = (roomState || {}).messages || []

    const onClickSend = useCallback(async () => {
        await api.sendMessage(messageText, nodeAddress, server, room, messages)
    }, [messageText, nodeAddress, server, room, messages, api.sendMessage])

    function onChangeMessageText(e) {
        setMessageText(e.target.value)
    }

    if (!stateURI) {
        return <div className={className}></div>
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
                <Input onChange={onChangeMessageText} value={messageText} />
                <Button onClick={onClickSend}>Send</Button>
            </ControlsContainer>
        </Container>
    )
}

export default Chat
