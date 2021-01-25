import React, { useState, useCallback, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Avatar } from '@material-ui/core'
import { SendRounded as SendIcon, AddCircleRounded as AddIcon } from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import rpcFetch from '../utils/rpcFetch'
import Sidebar from './Sidebar'
import Button from './Button'
import Input from './Input'
import useAPI from '../hooks/useAPI'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'
import strToColor from '../utils/strToColor'

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
    padding: ${props => props.firstByUser ? '20px 0 0' : '0'};
`

const MessageSender = styled.div`
    font-weight: 500;
`

const MessageText = styled.div`
`

const ControlsContainer = styled.div`
    display: flex;
    align-self: end;
    padding-bottom: 6px;
    width: 100%;
    margin-top: 6px;
`

const MessageInput = styled(Input)`
    padding-left: 34px;
    font-family: 'Noto Sans KR';
    font-size: 14px;
`

const SIconButton = styled(IconButton)`
    color: ${props => props.theme.color.white} !important;
    padding: 0 12px !important;
`

const HiddenInput = styled.input`
    opacity: 0;
    width: 1px;
`

const AddAttachmentButton = styled(AddIcon)`
    position: absolute;
    cursor: pointer;
    margin-top: 3px;
    margin-left: 3px;
`

const UserAvatar = styled(Avatar)`
    background-color: ${props => tinycolor(strToColor(props.sender)).darken(10).desaturate(25)} !important;
    font-weight: 700;
`

const UserAvatarPlaceholder = styled.div`
    width: 40px;
`

const MessageDetails = styled.div`
    display: flex;
    flex-direction: column;
    padding-left: 14px;
`

function Chat({ stateURI, className }) {
    const { nodeAddress } = useRedwood()
    const api = useAPI()
    const roomState = useStateTree(stateURI)
    const [messageText, setMessageText] = useState('')
    const attachmentInput = useRef()
    const theme = useTheme()

    let server, room
    if (!!stateURI) {
        ([server, room] = stateURI.split('/'))
    }
    let messages = (roomState || {}).messages || []

    const onClickSend = useCallback(async () => {
        if (!api) { return }
        console.log('attachmentInput', attachmentInput)
        await api.sendMessage(messageText, attachmentInput.current, nodeAddress, server, room, messages)
    }, [messageText, nodeAddress, server, room, messages, api])

    function onChangeMessageText(e) {
        setMessageText(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter') {
            onClickSend()
            setMessageText('')
        }
    }

    function onClickAddAttachment() {
        attachmentInput.current.click()
    }

    let previousSender
    messages = messages.map(msg => {
        msg = {
            ...msg,
            firstByUser: previousSender !== msg.sender,
        }
        previousSender = msg.sender
        return msg
    })

    if (!stateURI) {
        return <Container className={className}></Container>
    }
    return (
        <Container className={className}>
            <MessageContainer>
                {messages.map(msg => (
                    <Message firstByUser={msg.firstByUser}>
                        {msg.firstByUser  && <UserAvatar sender={msg.sender}>{(msg.sender || '').slice(0, 1).toUpperCase()}</UserAvatar>}
                        {!msg.firstByUser && <UserAvatarPlaceholder />}
                        <MessageDetails>
                            {msg.firstByUser && <MessageSender>{msg.sender}</MessageSender>}
                            <MessageText>{msg.text}</MessageText>
                        </MessageDetails>
                    </Message>
                ))}
            </MessageContainer>

            <ControlsContainer>
                <HiddenInput type="file" ref={attachmentInput} />
                <AddAttachmentButton onClick={onClickAddAttachment} style={{ color: theme.color.white }} />
                <MessageInput onKeyDown={onKeyDown} onChange={onChangeMessageText} value={messageText} />
                <SIconButton onClick={onClickSend}><SendIcon /></SIconButton>
            </ControlsContainer>
        </Container>
    )
}

export default Chat
