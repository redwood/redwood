import 'emoji-mart/css/emoji-mart.css'
import React, {
    useState,
    useCallback,
    useRef,
    useEffect,
    useMemo,
    memo,
} from 'react'
import styled, { useTheme } from 'styled-components'
import { IconButton, Tooltip } from '@material-ui/core'
import {
    SendRounded as SendIcon,
    AddCircleRounded as AddIcon,
} from '@material-ui/icons'
import * as tinycolor from 'tinycolor2'
import moment from 'moment'
import CloseIcon from '@material-ui/icons/Close'
import EmojiEmotionsIcon from '@material-ui/icons/EmojiEmotions'
import { Picker, Emoji } from 'emoji-mart'
import data from 'emoji-mart/data/all.json'
import { Node, createEditor, Editor, Transforms, Range } from 'slate'
import { withReact, ReactEditor } from 'slate-react'
import { withHistory } from 'slate-history'
import useRedwood from '../hooks/useRedwood'
import useStateTree from '../hooks/useStateTree'

import Button from './Button'
import Input from './Input'
import EmojiQuickSearch from './EmojiQuickSearch'
import TextBox from './TextBox'
import Message from './NewChat/Message'
import MessageList from './NewChat/MessageList'
import NormalizeMessage from './Chat/NormalizeMessage'
import AttachmentPreviewModal from './Modal/AttachmentPreviewModal'
import UserAvatar from './UserAvatar'
import Attachment from './Attachment'
import useModal from '../hooks/useModal'
import useServerRegistry from '../hooks/useServerRegistry'
import useAPI from '../hooks/useAPI'
import { isImage } from '../utils/contentTypes'
import useNavigation from '../hooks/useNavigation'
import useAddressBook from '../hooks/useAddressBook'
import useUsers from '../hooks/useUsers'
import emojiSheet from '../assets/emoji-mart-twitter-images.png'
import uploadIcon from '../assets/upload.svg'
import fileIcon from './Attachment/file.svg'
// import strToColor from '../utils/strToColor'

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

/* eslint-disable */
const withMentions = (editor) => {
    const { isInline, isVoid } = editor

    editor.isInline = (element) =>
        element.type === 'mention' ? true : isInline(element)

    editor.isVoid = (element) =>
        element.type === 'mention' ? true : isVoid(element)

    return editor
}

const withEmojis = (editor) => {
    const { isInline, isVoid } = editor

    editor.isInline = (element) =>
        element.type === 'emoji' ? true : isInline(element)

    editor.isVoid = (element) =>
        element.type === 'emoji' ? true : isVoid(element)

    return editor
}
/* eslint-enable */

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
    const [messageText, setMessageText] = useState(initialMessageText)
    // Emoji Variables
    const [emojiSearchWord, setEmojiSearchWord] = useState('')
    const [emojisFound, setEmojisFound] = useState(false)
    const [showEmojiKeyboard, setShowEmojiKeyboard] = useState(false)

    const theme = useTheme()
    const attachmentsInput = useRef()
    const attachmentForm = useRef()
    const newAttachmentsInput = useRef()
    // const messageTextContainer = useRef()
    const mentionRef = useRef()
    const controlsRef = useRef()
    const [attachments, setAttachments] = useState([])
    const [previews, setPreviews] = useState([])

    // Mention Variables
    const [targetMention, setTargetMention] = useState()
    const [indexMention, setIndexMention] = useState(0)
    const [searchMention, setSearchMention] = useState('')

    // const { onPresent: onPresentPreviewModal } = useModal('attachment preview')
    // const [previewedAttachment, setPreviewedAttachment] = useState({})
    // const onClickAttachment = useCallback(
    //     (attachment, url) => {
    //         setPreviewedAttachment({ attachment, url })
    //         onPresentPreviewModal()
    //     },
    //     [setPreviewedAttachment, onPresentPreviewModal],
    // )

    const numMessages = ((roomState || {}).messages || []).length
    const [messages, setMessages] = useState([])

    // Init Slate Editor
    // const editor = useMemo(() => withMentions(withHistory(withReact(createEditor()))), [])
    const editorRef = useRef()
    if (!editorRef.current)
        editorRef.current = withEmojis(
            withMentions(withHistory(withReact(createEditor()))),
        )
    const editor = editorRef.current

    const initFocusPoint = { path: [0, 0], offset: 0 }
    const [editorFocusPoint, setEditorFocusPoint] = useState(initFocusPoint)

    function onEditorBlur() {
        try {
            setEditorFocusPoint(editor.selection.focus)
        } catch (e) {
            console.error(e)
        }
    }

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

                return false
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
        if (targetMention && mentionUsers.length > 0) {
            const el = mentionRef.current
            const domRange = ReactEditor.toDOMRange(editor, targetMention)
            const rect = domRange.getBoundingClientRect()
            const topCalc = 68 + mentionUsers.length * 28

            el.style.top = `-${topCalc}px`
            el.style.left = '0px'
        }
    }, [
        mentionUsers.length,
        editor,
        indexMention,
        searchMention,
        targetMention,
    ])

    /* eslint-disable */
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
    /* eslint-enable */

    const onOpenEmojis = (event) => {
        if (showEmojiKeyboard) {
            setShowEmojiKeyboard(!showEmojiKeyboard)
        } else {
            setShowEmojiKeyboard(!showEmojiKeyboard)
        }
    }

    const onSelectEmoji = (emoji) => {
        if (typeof emoji === 'string') {
            const [start] = Range.edges(editor.selection)
            const wordBefore = Editor.before(editor, start, { unit: 'word' })
            const before = wordBefore && Editor.before(editor, wordBefore)
            const beforeRange = before && Editor.range(editor, before, start)
            Transforms.select(editor, {
                anchor: {
                    path: beforeRange.anchor.path,
                    offset:
                        beforeRange.focus.offset - emojiSearchWord.length - 1,
                },
                focus: beforeRange.focus,
            })

            const emojiNode = {
                type: 'emoji',
                value: emoji,
                children: [{ text: '' }],
            }
            Transforms.insertNodes(editor, [
                emojiNode,
                {
                    text: ' ',
                },
            ])
            Transforms.move(editor, { distance: 2 })
        } else {
            // Set focus point from blurred
            editor.selection = {
                anchor: editorFocusPoint,
                focus: editorFocusPoint,
            }
            const emojiNode = {
                type: 'emoji',
                value: emoji.colons,
                children: [{ text: '' }],
            }
            Transforms.insertNodes(editor, emojiNode)
            Transforms.move(editor, { distance: 1 })

            ReactEditor.focus(editor)
            setShowEmojiKeyboard(false)
        }
    }

    /* eslint-disable */
    function getCurrentEmojiWord(startPos, text) {
        if (text[startPos] === ' ' || text[startPos] === ':' || startPos < 2) {
            setEmojiSearchWord('')
            return
        }

        let cursor = startPos
        const searchWord = []

        while (true) {
            if (text[cursor] === ':' && cursor === 0) {
                setEmojiSearchWord(searchWord.reverse().join(''))
                break
            }

            if (text[cursor] === ':' && text[cursor - 1] !== ' ') {
                setEmojiSearchWord('')
                break
            }

            if (cursor === 0 || text[cursor] === ' ') {
                setEmojiSearchWord('')
                break
            }

            if (
                text[cursor] === ':' &&
                text[cursor - 1] === ' ' &&
                searchWord.length >= 2
            ) {
                setEmojiSearchWord(searchWord.reverse().join(''))
                break
            }

            if (text[cursor] !== ':') {
                searchWord.push(text[cursor].toLowerCase())
            }
            cursor--
        }

        return searchWord
    }
    /* eslint-enable */

    /* eslint-disable */
    const insertMention = (editor, selectedUser) => {
        const mention = {
            type: 'mention',
            selectedUser,
            children: [{ text: '' }],
        }
        Transforms.insertNodes(editor, [
            mention,
            {
                text: ' ',
            },
        ])
        Transforms.move(editor, { distance: 2 })
        setMessageText(editor.children)
    }
    /* eslint-enable */

    const onClickSend = useCallback(async () => {
        const plainMessage = serializeMessageText()
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
    ])

    // useEffect(() => {
    //     // Scrolls on new messages
    //     if (messageTextContainer.current) {
    //         setTimeout(() => {
    //             if (messageTextContainer.current !== null) {
    //                 messageTextContainer.current.scrollTop =
    //                     messageTextContainer.current.scrollHeight
    //             }
    //         }, 0)
    //     }
    // }, [numMessages])

    const serializeMessageText = useCallback(() => {
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
    }, [messageText])

    function onChangeMessageText(textValue) {
        setMessageText(textValue)
        const { selection } = editor

        if (selection && Range.isCollapsed(selection)) {
            const [start] = Range.edges(selection)
            const wordBefore = Editor.before(editor, start, { unit: 'word' })
            const before = wordBefore && Editor.before(editor, wordBefore)
            const beforeRange = before && Editor.range(editor, before, start)
            const beforeText = beforeRange && Editor.string(editor, beforeRange)
            const beforeMatch = beforeText && beforeText.match(/^@(\w+)$/)
            const after = Editor.after(editor, start)
            const afterRange = Editor.range(editor, start, after)
            const afterText = Editor.string(editor, afterRange)
            const afterMatch = afterText.match(/^(\s|$)/)

            if (beforeMatch && afterMatch) {
                setTargetMention(beforeRange)
                setSearchMention(beforeMatch[1])
                setIndexMention(0)
                return
            }
        }

        setTargetMention(null)

        if (selection) {
            const { anchor, focus } = editor.selection

            // Assure cursor isn't selecting text
            if (
                anchor.offset === focus.offset &&
                JSON.stringify(anchor.path) === JSON.stringify(focus.path)
            ) {
                const parentNode = editor.children[focus.path[0]]
                const childNode = parentNode.children[focus.path[1]]

                if (childNode.text) {
                    setTimeout(() => {
                        getCurrentEmojiWord(focus.offset - 1, childNode.text)
                    }, 0)
                }
            }
        }
    }

    const onKeyDown = useCallback(
        (event) => {
            if (targetMention && mentionUsers.length > 0) {
                switch (event.key) {
                    case 'ArrowDown': {
                        event.preventDefault()
                        const prevIndex =
                            indexMention >= mentionUsers.length - 1
                                ? 0
                                : indexMention + 1
                        setIndexMention(prevIndex)
                        break
                    }
                    case 'ArrowUp': {
                        event.preventDefault()
                        const nextIndex =
                            indexMention <= 0
                                ? mentionUsers.length - 1
                                : indexMention - 1
                        setIndexMention(nextIndex)
                        break
                    }
                    case 'Tab':
                    case 'Enter': {
                        const selectedUser = mentionUsers[indexMention]
                        if (selectedUser) {
                            event.preventDefault()

                            Transforms.select(editor, targetMention)

                            insertMention(editor, selectedUser)
                            setTargetMention(null)
                        }
                        break
                    }
                    case 'Escape': {
                        event.preventDefault()
                        setTargetMention(null)
                        break
                    }
                    default:
                        return
                }
            } else if (event.code === 'Enter' && !event.shiftKey) {
                // Emoji Handling
                if (!emojisFound || (!emojisFound && emojiSearchWord)) {
                    event.preventDefault()
                    event.stopPropagation()

                    onClickSend()
                }
            }

            if (emojiSearchWord) {
                if (event.code === 'ArrowUp' || event.code === 'ArrowDown') {
                    event.preventDefault()
                }
            }
        },
        [
            messageText,
            emojisFound,
            indexMention,
            searchMention,
            targetMention,
            attachments,
            previews,
        ],
    )

    const onClickAddAttachment = useCallback(() => {
        if (previews.length === 0) {
            attachmentsInput.current.value = ''
            attachmentForm.current.reset()
        }
        attachmentsInput.current.click()
    }, [previews.length, attachmentForm, attachmentsInput])

    function onClickAddNewAttachment() {
        newAttachmentsInput.current.click()
    }

    /* eslint-disable */
    function removePreview(itemIdx) {
        const clonedPreviews = [...previews]
        const clonedAttachments = [...attachments]
        clonedPreviews.splice(itemIdx, 1)
        clonedAttachments.splice(itemIdx, 1)
        setPreviews(clonedPreviews)
        setAttachments(clonedAttachments)
        if (clonedPreviews.length === 0) {
            attachmentsInput.current.value === ''
            attachmentForm.current.reset()
        }
    }
    /* eslint-enable */

    function onChangeAttachments(event) {
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
    }
    /* eslint-enable */

    function addNewAttachment(event) {
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
    }

    if (!selectedStateURI) {
        return (
            <EmptyChatContainer className={className}>
                Please select a server and a chat to get started!
            </EmptyChatContainer>
        )
    }

    /* eslint-disable */
    return (
        <Container className={className}>
            <MessageList messages={messages} nodeIdentities={nodeIdentities} />
            {/* <MessageContainer ref={messageTextContainer}>
                {messages.map((msg, i) => (
                    <Message
                        msg={msg}
                        ownAddress={ownAddress}
                        onClickAttachment={onClickAttachment}
                        messageIndex={i}
                        key={i}
                    />
                ))}
            </MessageContainer> */}

            {/* <AttachmentPreviewModal
                attachment={previewedAttachment.attachment}
                url={previewedAttachment.url}
            /> */}

            <ImgPreviewContainer show={previews.length > 0}>
                {previews.map((dataURL, idx) =>
                    dataURL ? (
                        <SImgPreviewWrapper key={idx}>
                            <button onClick={() => removePreview(idx)}>
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
                {/* <MessageInput ref={messageInputRef} onKeyDown={onKeyDown} onChange={onChangeMessageText} value={messageText} /> */}
                <TextBox
                    onKeyDown={onKeyDown}
                    onChange={onChangeMessageText}
                    value={messageText}
                    editor={editor}
                    onBlur={onEditorBlur}
                    mentionUsers={mentionUsers}
                    indexMention={indexMention}
                    targetMention={targetMention}
                    mentionRef={mentionRef}
                    controlsRef={controlsRef}
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
                {showEmojiKeyboard ? (
                    <EmojiWrapper>
                        <Picker
                            useButton={false}
                            title="Redwood Chat"
                            backgroundImageFn={(set, sheetSize) => emojiSheet}
                            perLine={8}
                            set="twitter"
                            theme="dark"
                            emojiSize={24}
                            onSelect={onSelectEmoji}
                        />
                    </EmojiWrapper>
                ) : null}
                {emojiSearchWord ? (
                    <EmojiQuickSearch
                        emojisFound={emojisFound}
                        setEmojisFound={setEmojisFound}
                        setEmojiSearchWord={setEmojiSearchWord}
                        onSelectEmoji={onSelectEmoji}
                        messageText={emojiSearchWord}
                    />
                ) : null}
            </ControlsContainer>
        </Container>
    )
    /* eslint-enable */
}

function getTimestampDisplay(timestamp) {
    const momentDate = moment.unix(timestamp)
    let dayDisplay = momentDate.format('MM/DD')
    const displayTime = momentDate.format('h:mm A')

    if (
        momentDate.format('MM/DD') === moment.unix(Date.now()).format('MM/DD')
    ) {
        dayDisplay = 'Today'
    } else if (
        momentDate.subtract(1, 'day') ===
        moment.unix(Date.now()).subtract(1, 'day')
    ) {
        dayDisplay = 'Yesterday'
    }
    return {
        dayDisplay,
        displayTime,
    }
}

const MessageWrapper = styled.div`
    display: flex;
    padding: ${(props) => (props.firstByUser ? '20px 0 0' : '0')};
    border-radius: 8px;
    transition: 50ms all ease-in-out;
    // &:hover {
    //   background: ${(props) => props.theme.color.grey[300]};
    // }
`

const SUserAvatar = styled(UserAvatar)`
    cursor: pointer;
`

const MessageSender = styled.div`
    font-weight: 500;
`

const SMessageTimestamp = styled.span`
    font-size: 10px;
    font-weight: 300;
    color: rgba(255, 255, 255, 0.4);
    margin-left: 4px;
`

function MessageTimestamp({ dayDisplay, displayTime }) {
    return (
        <SMessageTimestamp>
            {dayDisplay} {displayTime}
        </SMessageTimestamp>
    )
}

const SAttachment = styled(Attachment)`
    max-width: 500px;
`

// function Message({ msg = {}, isOwnMessage, onClickAttachment, messageIndex }) {
//     const { selectedServer, selectedStateURI } = useNavigation()
//     const { users, usersStateURI } = useUsers(selectedStateURI)
//     const addressBook = useAddressBook()
//     const userAddress = msg.sender.toLowerCase()
//     const user = (users && users[userAddress]) || {}
//     const displayName = addressBook[userAddress] || user.username || msg.sender
//     const { dayDisplay, displayTime } = getTimestampDisplay(msg.timestamp)
//     const { onPresent: onPresentContactsModal } = useModal('contacts')
//     const { onPresent: onPresentUserProfileModal } = useModal('user profile')
//     const { httpHost } = useRedwood()

//     const showContactsModal = useCallback(() => {
//         if (isOwnMessage) {
//             onPresentUserProfileModal()
//         } else {
//             onPresentContactsModal({ initiallyFocusedContact: msg.sender })
//         }
//     }, [
//         onPresentContactsModal,
//         onPresentUserProfileModal,
//         msg.sender,
//         isOwnMessage,
//     ])

//     /* eslint-disable */
//     return (
//         <MessageWrapper
//             firstByUser={msg.firstByUser}
//             key={selectedStateURI + messageIndex}
//         >
//             {msg.firstByUser ? (
//                 <SUserAvatar
//                     address={userAddress}
//                     onClick={showContactsModal}
//                 />
//             ) : (
//                 <UserAvatarPlaceholder />
//             )}
//             <MessageDetails>
//                 {msg.firstByUser && (
//                     <MessageSender>
//                         {displayName}{' '}
//                         <MessageTimestamp
//                             dayDisplay={dayDisplay}
//                             displayTime={displayTime}
//                         />
//                     </MessageSender>
//                 )}

//                 {/* <MessageText>{msg.text}</MessageText> */}
//                 {/* <MessageParse msgText={msg.text} /> */}
//                 <NormalizeMessage msgText={msg.text} />
//                 {(msg.attachments || []).map((attachment, j) => (
//                     <SAttachment
//                         key={`${selectedStateURI}${messageIndex},${j}`}
//                         attachment={attachment}
//                         url={`${httpHost}/messages[${messageIndex}]/attachments[${j}]?state_uri=${encodeURIComponent(
//                             selectedStateURI,
//                         )}`}
//                         onClick={onClickAttachment}
//                     />
//                 ))}
//             </MessageDetails>
//         </MessageWrapper>
//     )
//     /* eslint-enable */
// }

function MessageParse({ msgText }) {
    const colons = `:[a-zA-Z0-9-_+]+:`
    const skin = `:skin-tone-[2-6]:`
    const colonsRegex = new RegExp(`(${colons}${skin}|${colons})`, 'g')
    /* eslint-disable */
    const msgBlock = msgText
        .split(colonsRegex)
        .filter((block) => !!block)
        .map((block, idx) => {
            if (data.emojis[block.replace(':', '').replace(':', '')]) {
                if (block[0] === ':' && block[block.length - 1] === ':') {
                    return <Emoji key={idx} emoji={block} size={21} />
                }
            }

            return block
        })
    /* eslint-enable */

    return <SMessageParseContainer>{msgBlock}</SMessageParseContainer>
}

const SMessageParseContainer = styled.div`
    white-space: pre-wrap;
    span.emoji-mart-emoji {
        top: 4px;
    }
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

const EmojiWrapper = styled.div`
    position: absolute;
    top: -440px;
    right: 20px;
`

Chat.whyDidIRender = true

export default Chat
