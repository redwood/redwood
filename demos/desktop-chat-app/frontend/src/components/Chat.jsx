import 'emoji-mart/css/emoji-mart.css'
import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Tooltip } from '@material-ui/core'
import { SendRounded as SendIcon, AddCircleRounded as AddIcon } from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import filesize from 'filesize.js'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'
import EmojiEmotionsIcon from '@material-ui/icons/EmojiEmotions';
import { Picker, Emoji } from 'emoji-mart'

import Button from './Button'
import Input from './Input'
import Attachment from './Attachment'
import Embed from './Embed'
import Modal, { ModalTitle, ModalContent, ModalActions } from './Modal'
import UserAvatar from './UserAvatar'
import useModal from '../hooks/useModal'
import useServerRegistry from '../hooks/useServerRegistry'
import useAPI from '../hooks/useAPI'
import { useRedwood, useStateTree } from 'redwood-p2p-client/react'
import useNavigation from '../hooks/useNavigation'
import useAddressBook from '../hooks/useAddressBook'
import useUsers from '../hooks/useUsers'
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
    position: relative;
`

const MessageInput = styled(Input)`
    padding-left: 34px;
    font-family: 'Noto Sans KR';
    font-size: 14px;
`

const SIconButton = styled(IconButton)`
    color: ${props => props.theme.color.white} !important;
    padding: 0 8px !important;
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

const EmptyChatContainer = styled(Container)`
    display: flex;
    align-items: center;
    justify-content: center;
`

function Chat({ className }) {
    const { nodeIdentities } = useRedwood()
    const api = useAPI()
    const { selectedStateURI, selectedServer, selectedRoom } = useNavigation()
    const { users } = useUsers(selectedStateURI)
    const registry = useServerRegistry(selectedServer)
    const roomState = useStateTree(selectedStateURI)
    const [messageText, setMessageText] = useState('')
    const theme = useTheme()
    const attachmentsInput = useRef()
    const messageTextContainer = useRef()
    const [attachments, setAttachments] = useState([])
    const [previews, setPreviews] = useState([])
    const [showEmojiKeyboard, setShowEmojiKeyboard] = useState(false)

    const { onPresent: onPresentPreviewModal } = useModal('attachment preview')
    const [previewedAttachment, setPreviewedAttachment] = useState({})
    const onClickAttachment = useCallback((attachment, url) => {
        setPreviewedAttachment({ attachment, url })
        onPresentPreviewModal()
    }, [setPreviewedAttachment, onPresentPreviewModal])

    const numMessages = ((roomState || {}).messages || []).length
    const [messages, setMessages] = useState([])
    useEffect(() => {
        let previousSender
        let messages = ((roomState || {}).messages || []).map(msg => {
            msg = {
                ...msg,
                firstByUser: previousSender !== msg.sender,
                attachment: ((msg.attachment || {}).value || {}).value
            }
            previousSender = msg.sender
            return msg
        })
        setMessages(messages)
    }, [numMessages])

    const onOpenEmojis = () => {
      setShowEmojiKeyboard(!showEmojiKeyboard)
    }

    const onSelectEmoji = (emoji) => {
      setMessageText(messageText + emoji.colons)
    }

    const onClickSend = useCallback(async () => {
        if (!api || messageText.trim() === '') { return }
        await api.sendMessage(messageText, attachments, nodeIdentities[0].address, selectedServer, selectedRoom, messages)
        setAttachments([])
        setPreviews([])
    }, [messageText, nodeIdentities, attachments, selectedServer, selectedRoom, messages, api])

    useEffect(() => {
        // Scrolls on new messages
        if (messageTextContainer.current) {
            messageTextContainer.current.scrollTop = messageTextContainer.current.scrollHeight
        }
    })

    function onChangeMessageText(e) {
        setMessageText(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter' && messageText.trim() !== '') {
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

    if (!selectedStateURI) {
        return <EmptyChatContainer className={className}>
          Please select a server and a chat to get started!
        </EmptyChatContainer>
    }

    const ownAddress = nodeIdentities && nodeIdentities[0] ? nodeIdentities[0].address : null

    return (
        <Container className={className}>
            <MessageContainer ref={messageTextContainer}>
                {messages.map((msg, i) => (
                    <Message
                        msg={msg}
                        isOwnMessage={msg.sender === ownAddress}
                        onClickAttachment={onClickAttachment}
                        messageIndex={i}
                        key={msg.sender + msg.timestamp + i}
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
                { showEmojiKeyboard ? <EmojiWrapper>
                  <Picker
                    useButton={false}
                    title={'Redwood Chat'}
                    perLine={8}
                    set='apple'
                    theme='dark'
                    emojiSize={24}
                    onSelect={onSelectEmoji}
                  />
                </EmojiWrapper> : null }
                <SIconButton onClick={onOpenEmojis}><EmojiEmotionsIcon /></SIconButton>
                <SIconButton onClick={onClickSend}><SendIcon /></SIconButton>
            </ControlsContainer>
        </Container>
    )
}

const EmojiWrapper = styled.div`
  position: absolute;
  top: -440px;
  right: 20px;
`

const MessageWrapper = styled.div`
    display: flex;
    padding: ${props => props.firstByUser ? '20px 0 0' : '0'};
    border-radius: 8px;
    transition: 50ms all ease-in-out;
    // &:hover {
    //   background: ${props => props.theme.color.grey[300]};
    // }
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

function MessageTimestamp({ dayDisplay, displayTime }) {
    return <SMessageTimestamp>{dayDisplay} {displayTime}</SMessageTimestamp>
}

function getTimestampDisplay(timestamp) {
    const momentDate = moment.unix(timestamp)
    let dayDisplay = momentDate.format('MM/DD')
    let displayTime = momentDate.format('h:mm A')

    if (momentDate.format('MM/DD') === moment.unix(Date.now()).format('MM/DD')){
        dayDisplay = 'Today'
    } else if (momentDate.subtract(1, 'day') === moment.unix(Date.now()).subtract(1, 'day')) {
        dayDisplay = 'Yesterday'
    }
    return {
        dayDisplay,
        displayTime,
    }
}

const SUserAvatar = styled(UserAvatar)`
    cursor: pointer;
`

function Message({ msg, isOwnMessage, onClickAttachment, messageIndex }) {
    let { selectedServer, selectedStateURI } = useNavigation()
    let { users, usersStateURI } = useUsers(selectedStateURI)
    let addressBook = useAddressBook()
    let userAddress = msg.sender.toLowerCase()
    let user = (users && users[userAddress]) || {}
    let displayName = addressBook[userAddress] || user.username || msg.sender
    let { dayDisplay, displayTime } = getTimestampDisplay(msg.timestamp)
    let { onPresent: onPresentContactsModal } = useModal('contacts')
    let { onPresent: onPresentUserProfileModal } = useModal('user profile')
    let { httpHost } = useRedwood()

    let showContactsModal = useCallback(() => {
        if (isOwnMessage) {
            onPresentUserProfileModal()
        } else {
            onPresentContactsModal({ initiallyFocusedContact: msg.sender })
        }
    }, [onPresentContactsModal, onPresentUserProfileModal, msg, msg && msg.sender, isOwnMessage])

    return (
        <MessageWrapper firstByUser={msg.firstByUser} key={selectedStateURI + messageIndex}>
          {msg.firstByUser
              ? <SUserAvatar address={userAddress} onClick={showContactsModal} />
              : <UserAvatarPlaceholder />
          }
          <MessageDetails>
            {msg.firstByUser &&
              <MessageSender>{displayName} <MessageTimestamp dayDisplay={dayDisplay} displayTime={displayTime} /></MessageSender>
            }

            {/* <MessageText>{msg.text}</MessageText> */}
            <MessageParse msgText={msg.text} />
            {(msg.attachments || []).map((attachment, j) => (
              <SAttachment
                key={`${selectedStateURI}${messageIndex},${j}`}
                attachment={attachment}
                url={`${httpHost}/messages[${messageIndex}]/attachments[${j}]?state_uri=${encodeURIComponent(selectedStateURI)}`}
                onClick={onClickAttachment}
              />
            ))}
          </MessageDetails>
        </MessageWrapper>
    )
}

function MessageParse({ msgText }) {

  const colons = `:[a-zA-Z0-9-_+]+:`;
  const skin = `:skin-tone-[2-6]:`;
  const colonsRegex = new RegExp(`(${colons}${skin}|${colons})`, 'g');

  let msgBlock = msgText.split(colonsRegex).filter((block) => !!block).map((block) => {
    if (block[0] === ':' && block[block.length - 1] === ':') {
      return <Emoji emoji={block} size={18} />
    }

    return <span style={{ whiteSpace: 'pre-wrap' }}>{block}</span>
  })

  return (
    <SMessageParseContainer>{msgBlock}</SMessageParseContainer>
  )
}

const SMessageParseContainer = styled.div`
  // display: flex;
  // align-items: center
`

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
