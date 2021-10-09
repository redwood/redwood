import 'emoji-mart/css/emoji-mart.css'
import React, {
    useState,
    useCallback,
    useRef,
    useEffect,
    Fragment,
} from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Tooltip } from '@material-ui/core'
import {
    SendRounded as SendIcon,
    AddCircleRounded as AddIcon,
} from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import filesize from 'filesize.js'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'
import EmojiEmotionsIcon from '@material-ui/icons/EmojiEmotions'
import { Picker, Emoji } from 'emoji-mart'
import data from 'emoji-mart/data/all.json'
import { Node, createEditor, Editor, Transforms, Range } from 'slate'
import { withReact, ReactEditor } from 'slate-react'
import { withHistory } from 'slate-history'
import { useRedwood, useStateTree } from '@redwood.dev/client/react'

import Button from '../Button'
import Input from '../Input'
import Embed from '../Embed'
import TextBox from '../TextBox'
import Message from './Message'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import UserAvatar from '../UserAvatar'
import AttachmentPreviewModal from '../AttachmentPreviewModal'
import ImgPreviewContainer from './ImgPreviewContainer'
import useModal from '../../hooks/useModal'
import useServerRegistry from '../../hooks/useServerRegistry'
import useAPI from '../../hooks/useAPI'
import { isImage } from '../../utils/contentTypes'
import useNavigation from '../../hooks/useNavigation'
import useAddressBook from '../../hooks/useAddressBook'
import useUsers from '../../hooks/useUsers'
import emojiSheet from '../../assets/emoji-mart-twitter-images.png'
import downloadIcon from '../../assets/download.svg'
import cancelIcon from '../../assets/cancel-2.svg'
import uploadIcon from '../../assets/upload.svg'
import fileIcon from '../Attachment/file.svg'
// import strToColor from '../utils/strToColor'

import { getCurrentEmojiWord } from './utils'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    // height: 100%;
    flex-grow: 1;
    background-color: ${(props) => props.theme.color.grey[200]};
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
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

const MessageInput = styled(Input)`
    padding-left: 34px;
    font-family: 'Noto Sans KR';
    font-size: 14px;
`

const SIconButton = styled(IconButton)`
    color: ${(props) => props.theme.color.white} !important;
    padding: 0 8px !important;
    height: 100%;
`

const HiddenInput = styled.form`
    opacity: 0;
    width: 1px;
    display: inline-block;
    input {
        opacity: 0;
        width: 1px;
    }
`

const AdditionHiddenInput = styled.input`
    opacity: 0;
    width: 1px;
`

const AddAttachmentButton = styled(AddIcon)`
    position: absolute;
    cursor: pointer;
    margin-top: 4px;
    margin-left: 4px;
    z-index: 999;
    left: 12px;
    bottom: 22px;
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

    const theme = useTheme()
    const messageTextContainer = useRef()

    // Mention Variables
    const [targetMention, setTargetMention] = useState()
    const [indexMention, setIndexMention] = useState(0)
    const [searchMention, setSearchMention] = useState('')

    const attachmentsInputRef = useRef()
    const attachmentFormRef = useRef()

    const { onPresent: onPresentPreviewModal } = useModal('attachment preview')
    const [previewedAttachment, setPreviewedAttachment] = useState({})
    const onClickAttachment = useCallback(
        (attachment, url) => {
            setPreviewedAttachment({ attachment, url })
            onPresentPreviewModal()
        },
        [setPreviewedAttachment, onPresentPreviewModal],
    )

    const numMessages = ((roomState || {}).messages || []).length
    const [messages, setMessages] = useState([])

    const initFocusPoint = { path: [0, 0], offset: 0 }
    const [editorFocusPoint, setEditorFocusPoint] = useState(initFocusPoint)

    const onEditorBlur = useCallback(() => {
        try {
            setEditorFocusPoint(editor.selection.focus)
        } catch (e) {
            console.error(e)
        }
    }, [setEditorFocusPoint, editor])

    let mentionUsers = []
    const userAddresses = Object.keys(users || {})
    if (userAddresses.length) {
        mentionUsers = userAddresses
            .map((address) => ({ ...users[address], address }))
            .filter((user) => {
                if (!user.username && !user.nickname) {
                    return user.address.includes(searchMention.toLowerCase())
                }

                if (user.username) {
                    return user.username
                        .toLowerCase()
                        .includes(searchMention.toLowerCase())
                }

                if (user.nickname) {
                    return user.nickname
                        .toLowerCase()
                        .includes(searchMention.toLowerCase())
                }
            })
            .slice(0, 10)
    }

    useEffect(async () => {
        if (nodeIdentities) {
            if (Object.keys(users || {}).length > 0) {
                if (!users[nodeIdentities[0].address]) {
                    await api.updateProfile(
                        nodeIdentities[0].address,
                        `${selectedServer}/registry`,
                        null,
                        null,
                        'member',
                    )
                }
            }
        }
    }, [selectedStateURI, users, nodeIdentities])

    useEffect(() => {
        let previousSender
        const messages = ((roomState || {}).messages || []).map((msg) => {
            msg = {
                ...msg,
                firstByUser: previousSender !== msg.sender,
                attachment: ((msg.attachment || {}).value || {}).value,
            }
            previousSender = msg.sender
            return msg
        })
        setMessages(messages)
    }, [numMessages])

    useEffect(() => {
        // Scrolls on new messages
        if (messageTextContainer.current) {
            setTimeout(() => {
                if (messageTextContainer.current !== null) {
                    messageTextContainer.current.scrollTop =
                        messageTextContainer.current.scrollHeight
                }
            }, 0)
        }
    }, [numMessages])

    if (!selectedStateURI) {
        return (
            <EmptyChatContainer className={className}>
                Please select a server and a chat to get started!
            </EmptyChatContainer>
        )
    }

    const ownAddress =
        nodeIdentities && nodeIdentities[0] ? nodeIdentities[0].address : null

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

            <AttachmentPreviewModal
                attachment={previewedAttachment.attachment}
                url={previewedAttachment.url}
            />

            <ImgPreviewContainer
                attachmentsInputRef={attachmentsInputRef}
                attachmentFormRef={attachmentFormRef}
            />
            <Controls
                attachmentsInputRef={attachmentsInputRef}
                attachmentFormRef={attachmentFormRef}
            />
        </Container>
    )
}

export default Chat
