import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Tooltip } from '@material-ui/core'
import { SendRounded as SendIcon, AddCircleRounded as AddIcon } from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import filesize from 'filesize.js'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'

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
    // height: 100%;
    flex-grow: 1;
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
    padding-left: 40px;
    // width: 40px;
`

const MessageDetails = styled.div`
    display: flex;
    flex-direction: column;
    padding-left: 14px;
    padding-bottom: 6px;
`

const SAttachment = styled(Attachment)`
    max-width: 200px;
`

const ImgPreviewContainer = styled.div`
    height: ${props => props.show ? 'unset' : '0px'};
`

const ImgPreview = styled.img`
    height: 100px;
    border: 1px dashed rgba(255, 255, 255, .5);
    padding: 4px;
    margin: 3px;
`

const SImgPreviewWrapper = styled.div`
    position: relative;
    display: inline-block;
    margin-right: 12px;
    button {
      cursor: pointer;
      border: none;
      position: absolute;
      top: -4px;
      right: -4px;
      border-radius: 100%;
      height: 24px;
      width: 24px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: ${props => props.theme.color.indigo[500]};
      transition: all ease-in-out .15s;
      outline: none;
      &:hover {
        transform: scale(1.1);
      }
      svg {
        color: white;
        height: 18px;
      }
    }
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
    const messageTextContainer = useRef()
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
        console.log('attachments', attachments)
        await api.sendMessage(messageText, attachments, nodeAddress, selectedServer, selectedRoom, messages)
        setAttachments([])
        setPreviews([])
    }, [messageText, nodeAddress, attachments, selectedServer, selectedRoom, messages, api])

    useEffect(() => {
      // Scrolls on new messages
      if (messageTextContainer.current) {
        messageTextContainer.current.scrollTop = messageTextContainer.current.scrollHeight
      }
    }, [roomState])

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

    function removePreview(itemIdx) {
      let clonedPreviews = [...previews]
      clonedPreviews.splice(itemIdx, 1)
      setPreviews(clonedPreviews)
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
            <MessageContainer ref={messageTextContainer}>
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
                {previews.map((dataURL, idx) => !!dataURL ? (
                  <SImgPreviewWrapper key={idx}>
                    <button onClick={() => removePreview(idx)}>
                      <CloseIcon />
                    </button>
                    <ImgPreview src={dataURL} key={dataURL} />
                  </SImgPreviewWrapper>
                ) : null)}
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
    cursor: pointer;
    transition: .1s all ease-in-out;
    &:hover {
      background: rgba(0,0,0, .12);
    }
`

const MessageSender = styled.div`
    font-weight: 500;
`

const MessageText = styled.div`
`

const SMessageTimestamp = styled.span`
  font-size: 10px;
  font-weight: 300;
  color: rgba(255,255,255, .4);
  margin-left: 4px;
`

function MessageTimeStamp({ dayDisplay, displayTime }) {

  return (
    <SMessageTimestamp>{dayDisplay} {displayTime}</SMessageTimestamp>
  )
}

function getTimestampDisplay(timestamp) {
  const momentDate = moment.unix(timestamp)
  let dayDisplay = momentDate.format('MM/YY')
  let displayTime = momentDate.format('h:mm A')

  if (momentDate.format('MM/YY') === moment().format('MM/YY')){
    dayDisplay = 'Today'
  } else if (momentDate.subtract(1, 'day') === moment().subtract(1, 'day')) {
    dayDisplay = 'Yesterday'
  }

  return {
    dayDisplay,
    displayTime,
  }

}

function Message({ msg, user, selectedServer, selectedStateURI, onClickAttachment, messageIndex }) {
    user = user || {}
    let userAddress = msg.sender.toLowerCase()
    let displayName = user.username || msg.sender
    const { dayDisplay, displayTime } = getTimestampDisplay(msg.timestamp)

    return (
      <Tooltip title={`${dayDisplay} ${displayTime}`} placement="left" arrow>
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
              <MessageSender>{displayName} <MessageTimeStamp dayDisplay={dayDisplay} displayTime={displayTime} /></MessageSender>
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
      </Tooltip>
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
