import React, { useState, useCallback, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Avatar } from '@material-ui/core'
import { SendRounded as SendIcon, AddCircleRounded as AddIcon } from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import filesize from 'filesize.js'
import moment from 'moment'

import Button from './Button'
import Input from './Input'
import Attachment from './Attachment'
import Embed from './Embed'
import Modal, { ModalTitle, ModalContent, ModalActions } from './Modal'
import UserAvatar from './UserAvatar'
import useModal from '../hooks/useModal'
import useAPI from '../hooks/useAPI'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'
import useNavigation from '../hooks/useNavigation'
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
    margin-top: 4px;
    margin-left: 4px;
`

const UserAvatarPlaceholder = styled.div`
    width: 40px;
`

const MessageDetails = styled.div`
    display: flex;
    flex-direction: column;
    padding-left: 14px;
`

const SAttachment = styled(Attachment)`
    max-width: 200px;
`

const ImgPreviewContainer = styled.div`
    height: ${props => props.show ? 'unset' : '0px'};
`

const ImgPreview = styled.img`
    height: 100px;
    border: 1px solid white;
    margin: 3px;
`

function Chat({ className }) {
    const { nodeAddress } = useRedwood()
    const api = useAPI()
    const { selectedStateURI, selectedServer, selectedRoom } = useNavigation()
    const registry = useStateTree(!!selectedServer ? `${selectedServer}/registry` : null)
    const roomState = useStateTree(selectedStateURI)
    const [messageText, setMessageText] = useState('')
    const theme = useTheme()
    const attachmentsInput = useRef()
    const [attachments, setAttachments] = useState([])
    const [previews, setPreviews] = useState([])

    const { onPresent: onPresentPreviewModal } = useModal('attachment preview')
    const [previewedAttachment, setPreviewedAttachment] = useState({})
    const onClickAttachment = useCallback((attachment, url) => {
        setPreviewedAttachment({ attachment, url })
        onPresentPreviewModal()
    }, [setPreviewedAttachment, onPresentPreviewModal])

    let users = (registry || {}).users || {}
    let messages = (roomState || {}).messages || []

    const onClickSend = useCallback(async () => {
        if (!api) { return }
        await api.sendMessage(messageText, attachments, nodeAddress, selectedServer, selectedRoom, messages)
        setAttachments([])
        setPreviews([])
    }, [messageText, nodeAddress, selectedServer, selectedRoom, messages, api])

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
        attachmentsInput.current.click()
    }

    function onChangeAttachments() {
        if (!attachmentsInput || !attachmentsInput.current || !attachmentsInput.current.files || attachmentsInput.current.files.length === 0) {
            setAttachments([])
            setPreviews([])
            return
        }

        let files = Array.prototype.map.call(attachmentsInput.current.files, x => x)
        setAttachments(files)
        setPreviews(new Array(files.length))

        for (let i = 0; i < files.length; i++) {
            (function (i) {
                let file = files[i]
                const reader = new FileReader()
                reader.addEventListener('load', () => {
                    setPreviews(prev => {
                        prev[i] = reader.result
                        return [ ...prev ]
                    })
                }, false)
                reader.readAsDataURL(file)
            })(i)
        }
    }

    let previousSender
    messages = messages.map(msg => {
        msg = {
            ...msg,
            firstByUser: previousSender !== msg.sender,
            attachment: ((msg.attachment || {}).value || {}).value
        }
        previousSender = msg.sender
        return msg
    })

    if (!selectedStateURI) {
        return <Container className={className}></Container>
    }

    return (
        <Container className={className}>
            <MessageContainer>
                {messages.map((msg, i) => (
                    <Message
                        msg={msg}
                        user={users[msg.sender.toLowerCase()]}
                        selectedServer={selectedServer}
                        selectedStateURI={selectedStateURI}
                        onClickAttachment={onClickAttachment}
                        messageIndex={i}
                    />
                ))}
            </MessageContainer>

            <AttachmentPreviewModal attachment={previewedAttachment.attachment} url={previewedAttachment.url} />

            <ImgPreviewContainer show={previews.length > 0}>
                {previews.map(dataURL => !!dataURL ? <ImgPreview src={dataURL} key={dataURL} /> : null)}
                <HiddenInput type="file" multiple ref={attachmentsInput} onChange={onChangeAttachments} />
            </ImgPreviewContainer>
            <ControlsContainer>
                <AddAttachmentButton onClick={onClickAddAttachment} style={{ color: theme.color.white }} />
                <MessageInput onKeyDown={onKeyDown} onChange={onChangeMessageText} value={messageText} />
                <SIconButton onClick={onClickSend}><SendIcon /></SIconButton>
            </ControlsContainer>
        </Container>
    )
}

const MessageWrapper = styled.div`
    display: flex;
    padding: ${props => props.firstByUser ? '20px 0 0' : '0'};
`

const MessageSender = styled.div`
    font-weight: 500;
`

const MessageText = styled.div`
`

function Message({ msg, user, selectedServer, selectedStateURI, onClickAttachment, messageIndex }) {
    // console.log('user', user)
    console.log('msg', msg)
    user = user || {}
    let userAddress = msg.sender.toLowerCase()
    let displayName = user.username || msg.sender
    let displayTimestamp = moment.unix(msg.timestamp).format('MM/YY h:mm A')

    return (
        <MessageWrapper firstByUser={msg.firstByUser} key={userAddress + msg.timestamp}>
            {msg.firstByUser ? <UserAvatar
                                    address={userAddress}
                                    username={user.username}
                                    photoURL={!!user.photo ? `http://localhost:8080/users/${userAddress.toLowerCase()}/photo?state_uri=${selectedServer}/registry` : null}
                                />
                             : <UserAvatarPlaceholder />
            }
            <MessageDetails>
                {msg.firstByUser &&
                    <MessageSender>{displayName} - {displayTimestamp}</MessageSender>
                }
                
                <MessageText>{msg.text}</MessageText>
                {(msg.attachments || []).map((attachment, j) => (
                    <SAttachment
                        key={`${selectedStateURI}${messageIndex},${j}`}
                        attachment={attachment}
                        url={`http://localhost:8080/messages[${messageIndex}]/attachments[${j}]?state_uri=${encodeURIComponent(selectedStateURI)}`}
                        onClick={onClickAttachment}
                    />
                ))}
            </MessageDetails>
        </MessageWrapper>
    )
}

const SModalContent = styled(ModalContent)`
    width: 600px;
    flex-direction: column;
`

const Metadata = styled.div`
    padding-bottom: 4px;
`

const Filename = styled.span`
    font-size: 0.8rem;
`

const Filesize = styled.span`
    font-size: 0.8rem;
    color: ${props => props.theme.color.grey[100]};
`

function AttachmentPreviewModal({ attachment, url }) {
    if (!attachment) {
        return null
    }
    return (
        <Modal modalKey="attachment preview">
            <SModalContent>
                <Metadata>
                    <Filename>{attachment.filename} </Filename>
                    <Filesize>({filesize(attachment['Content-Length'])})</Filesize>
                </Metadata>
                <Embed contentType={attachment['Content-Type']} url={url} width={600} />
            </SModalContent>
        </Modal>
    )
}

export default Chat
