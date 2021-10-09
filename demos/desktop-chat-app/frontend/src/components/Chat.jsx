import 'emoji-mart/css/emoji-mart.css'
import { useState, useCallback, useRef, useEffect, useMemo } from 'react'
import styled from 'styled-components'
import { IconButton } from '@material-ui/core'
import {
    SendRounded as SendIcon,
    AddCircleRounded as AddIcon,
} from '@material-ui/icons'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'
import EmojiEmotionsIcon from '@material-ui/icons/EmojiEmotions'
import { Emoji } from 'emoji-mart'
import data from 'emoji-mart/data/all.json'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'

import Button from './Button'
import Input from './Input'
import TextBox from './TextBox'
import MessageList from './NewChat/MessageList'
import UserAvatar from './UserAvatar'
import Attachment from './Attachment'
import useAPI from '../hooks/useAPI'
import { isImage } from '../utils/contentTypes'
import useNavigation from '../hooks/useNavigation'
import useUsers from '../hooks/useUsers'
import uploadIcon from '../assets/upload.svg'
import fileIcon from './Attachment/file.svg'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    // height: 100%;
    flex-grow: 1;
    background-color: ${(props) => props.theme.color.grey[200]};
`

const ControlsContainer = styled.div`
    display: flex;
    align-self: end;
    padding-bottom: 6px;
    width: 100%;
    margin-top: 6px;
    position: relative;
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
    color: white;
`

const ImgPreviewContainer = styled.div`
    transition: 0.3s ease-in-out all;
    transform: ${(props) =>
        props.show ? 'translateY(0px)' : 'translateY(100px)'};
    height: ${(props) => (props.show ? 'unset' : '0px')};
    border: 1px solid rgba(255, 255, 255, 0.2);
    padding-top: 8px;
    margin-right: 18px;
    padding-left: 12px;
    background: #27282c;
    border-radius: 6px;
    display: flex;
    flex-wrap: wrap;
`

const ImgPreview = styled.img`
    height: 60px;
    padding: 4px;
    margin: 3px;
    display: inline-block;
`

const SImgPreviewWrapper = styled.div`
    border: 1px dashed rgba(255, 255, 255, 0.5);
    position: relative;
    margin-right: 12px;
    padding-bottom: 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    align-self: center;
    span {
        display: inline-block;
        font-size: 10px;
        max-width: 120px;
        text-overflow: ellipsis;
        padding-left: 4px;
        padding-right: 4px;
        overflow: hidden;
        text-align: center;
        white-space: nowrap;
    }
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
        background: ${(props) => props.theme.color.indigo[500]};
        transition: all ease-in-out 0.15s;
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

const SAddNewAttachment = styled.div`
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    margin-right: 12px;
    border: 1px dashed rgba(255, 255, 255, 0.5);
    max-height: 108.47px;
    margin: 3px;
    padding-left: 8px;
    padding-right: 8px;
    padding-bottom: 8px;
    > img {
        height: 60px;
        transform: scale(1.1);
    }
`

const EmptyChatContainer = styled(Container)`
    display: flex;
    align-items: center;
    justify-content: center;
`

const TextBoxButtonWrapper = styled.div`
    position: absolute;
    right: 20px;
    bottom: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 32px;
`

const initialMessageText = [
    {
        type: 'paragraph',
        children: [
            {
                text: '',
            },
        ],
    },
]

const serializeMessageText = (messageText) => {
    let isEmpty = true
    const filteredMessage = []
    messageText.forEach((node) => {
        if (node.children.length === 1) {
            let trimmedNode = {}
            if (typeof node.children[0].text === 'string') {
                trimmedNode = {
                    ...node,
                    children: [
                        {
                            ...node.children[0],
                            text: node.children[0].text.trim(),
                        },
                    ],
                }
            }

            if (
                JSON.stringify(initialMessageText[0]) ===
                JSON.stringify(trimmedNode)
            ) {
                if (!isEmpty) {
                    filteredMessage.push(trimmedNode)
                }
            } else {
                isEmpty = false
                if (JSON.stringify(trimmedNode) === '{}') {
                    filteredMessage.push(node)
                } else {
                    filteredMessage.push(trimmedNode)
                }
            }
        } else {
            filteredMessage.push(node)
        }
    })

    if (filteredMessage.length === 0) {
        return false
    }

    const jsonMessage = JSON.stringify(filteredMessage)
    return jsonMessage
}

function Chat({ className }) {
    const { nodeIdentities } = useRedwood()
    const api = useAPI()
    const { selectedStateURI, selectedServer, selectedRoom } = useNavigation()
    const { users } = useUsers(selectedStateURI)
    const roomState = useStateTree(selectedStateURI)
    const [messageText, setMessageText] = useState(initialMessageText)
    const [attachments, setAttachments] = useState([])
    const [previews, setPreviews] = useState([])

    // Emoji Variables
    const [showEmojiKeyboard, setShowEmojiKeyboard] = useState(false)
    const [emojiSearchWord, setEmojiSearchWord] = useState('')

    const attachmentsInput = useRef()
    const attachmentForm = useRef()
    const newAttachmentsInput = useRef()
    const controlsRef = useRef()

    const messages = useMemo(() => {
        let previousSender
        return ((roomState || {}).messages || []).map((msg) => {
            msg = {
                ...msg,
                firstByUser: previousSender !== msg.sender,
                attachment: ((msg.attachment || {}).value || {}).value,
            }
            previousSender = msg.sender
            return msg
        })
    }, [roomState])

    const editorRef = useRef({})
    const { current: editor } = editorRef

    const initFocusPoint = { path: [0, 0], offset: 0 }
    const [editorFocusPoint, setEditorFocusPoint] = useState(initFocusPoint)

    useEffect(() => {
        async function addMemberAddress() {
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
        }
        addMemberAddress()
    }, [selectedStateURI, users, nodeIdentities, api, selectedServer])

    const onOpenEmojis = () => {
        if (showEmojiKeyboard) {
            setShowEmojiKeyboard(!showEmojiKeyboard)
        } else {
            setShowEmojiKeyboard(!showEmojiKeyboard)
        }
    }

    const onClickSend = useCallback(async () => {
        const plainMessage = serializeMessageText(messageText)
        if (!api || (!plainMessage && attachments.length === 0)) {
            return
        }
        // Replace with markdown serializer
        await api.sendMessage(
            plainMessage,
            attachments,
            nodeIdentities[0].address,
            selectedServer,
            selectedRoom,
            messages,
        )
        setAttachments([])
        setPreviews([])
        setEmojiSearchWord('')

        // Reset SlateJS cursor
        const point = { path: [0, 0], offset: 0 }
        setEditorFocusPoint(point)
        editor.selection = { anchor: point, focus: point }
        editor.history = { redos: [], undos: [] }

        setMessageText(initialMessageText)

        attachmentsInput.current.value = ''
    }, [
        messageText,
        nodeIdentities,
        attachments,
        selectedServer,
        selectedRoom,
        messages,
        api,
        previews,
        editor,
    ])

    const onClickAddAttachment = useCallback(() => {
        if (previews.length === 0) {
            attachmentsInput.current.value = ''
            attachmentForm.current.reset()
        }
        attachmentsInput.current.click()
    }, [previews.length, attachmentForm, attachmentsInput])

    const onClickAddNewAttachment = useCallback(() => {
        newAttachmentsInput.current.click()
    }, [])

    const removePreview = useCallback(
        (itemIdx) => {
            const clonedPreviews = [...previews]
            const clonedAttachments = [...attachments]
            clonedPreviews.splice(itemIdx, 1)
            clonedAttachments.splice(itemIdx, 1)
            setPreviews(clonedPreviews)
            setAttachments(clonedAttachments)
            if (clonedPreviews.length === 0) {
                attachmentsInput.current.value = ''
                attachmentForm.current.reset()
            }
        },
        [attachments, previews],
    )

    const onChangeAttachments = useCallback(() => {
        if (
            !attachmentsInput ||
            !attachmentsInput.current ||
            !attachmentsInput.current.files ||
            attachmentsInput.current.files.length === 0
        ) {
            setAttachments([])
            setPreviews([])
            return
        }

        const files = Array.prototype.map.call(
            attachmentsInput.current.files,
            (x) => x,
        )
        setAttachments(files)
        setPreviews(new Array(files.length))
        /* eslint-disable */
        for (let i = 0; i < files.length; i++) {
            ;(function (i) {
                const file = files[i]
                if (isImage(file.type)) {
                    const reader = new FileReader()
                    reader.addEventListener(
                        'load',
                        () => {
                            setPreviews((prev) => {
                                prev[i] = reader.result
                                return [...prev]
                            })
                        },
                        false,
                    )
                    reader.readAsDataURL(file)
                } else {
                    setPreviews((prev) => {
                        prev[i] = 'file'
                        return [...prev]
                    })
                }
            })(i)
        }
    }, [])

    const addNewAttachment = useCallback(() => {
        const files = Array.prototype.map.call(
            newAttachmentsInput.current.files,
            (x) => x,
        )
        setAttachments([...attachments, ...files])
        /* eslint-disable */
        for (let i = 0; i < files.length; i++) {
            ;(function (i) {
                const file = files[i]
                const reader = new FileReader()
                reader.addEventListener(
                    'load',
                    () => {
                        setPreviews([...previews, reader.result])
                    },
                    false,
                )
                reader.readAsDataURL(file)
            })(i)
        }
        /* eslint-enable */
    }, [attachments, previews])

    if (!selectedStateURI) {
        return (
            <EmptyChatContainer className={className}>
                Please select a server and a chat to get started!
            </EmptyChatContainer>
        )
    }

    return (
        <Container className={className}>
            <MessageList messages={messages} nodeIdentities={nodeIdentities} />
            <ImgPreviewContainer show={previews.length > 0}>
                {previews.map((dataURL, idx) =>
                    dataURL ? (
                        <SImgPreviewWrapper key={idx}>
                            <button
                                type="button"
                                onClick={() => removePreview(idx)}
                            >
                                <CloseIcon />
                            </button>
                            <div
                                style={{
                                    width: '100%',
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                }}
                            >
                                <ImgPreview
                                    onError={(event) => {
                                        event.target.src = fileIcon
                                    }}
                                    src={dataURL}
                                    key={dataURL}
                                />
                            </div>

                            <div
                                style={{
                                    width: '100%',
                                    display: 'flex',
                                    alignItems: 'center',
                                    justifyContent: 'center',
                                }}
                            >
                                <span>
                                    {attachments[idx]
                                        ? attachments[idx].name
                                        : 'File'}
                                </span>
                            </div>
                        </SImgPreviewWrapper>
                    ) : null,
                )}
                <SAddNewAttachment>
                    <img src={uploadIcon} alt="Upload" />
                    <Button
                        primary
                        style={{
                            width: '108px',
                            marginTop: 12,
                            fontSize: 10,
                            padding: '3px 6px',
                            paddingTop: '4px',
                            lineHeight: '1.25',
                        }}
                        onClick={onClickAddNewAttachment}
                    >
                        Add File(s)
                    </Button>
                </SAddNewAttachment>
                <AdditionHiddenInput
                    type="file"
                    multiple
                    ref={newAttachmentsInput}
                    onChange={addNewAttachment}
                />
                <HiddenInput ref={attachmentForm}>
                    <input
                        type="file"
                        multiple
                        ref={attachmentsInput}
                        onChange={onChangeAttachments}
                    />
                </HiddenInput>
            </ImgPreviewContainer>
            <ControlsContainer ref={controlsRef}>
                <AddAttachmentButton onClick={onClickAddAttachment} />
                <TextBox
                    messageText={messageText}
                    setMessageText={setMessageText}
                    initialMessageText={initialMessageText}
                    attachments={attachments}
                    editorRef={editorRef}
                    nodeIdentity={nodeIdentities[0].address}
                    showEmojiKeyboard={showEmojiKeyboard}
                    setShowEmojiKeyboard={setShowEmojiKeyboard}
                    controlsRef={controlsRef}
                    onClickSend={onClickSend}
                    emojiSearchWord={emojiSearchWord}
                    setEmojiSearchWord={setEmojiSearchWord}
                    editorFocusPoint={editorFocusPoint}
                    setEditorFocusPoint={setEditorFocusPoint}
                >
                    <TextBoxButtonWrapper>
                        <SIconButton onClick={onOpenEmojis}>
                            <EmojiEmotionsIcon />
                        </SIconButton>
                        <SIconButton onClick={onClickSend}>
                            <SendIcon />
                        </SIconButton>
                    </TextBoxButtonWrapper>
                </TextBox>
            </ControlsContainer>
        </Container>
    )
}

export default Chat
