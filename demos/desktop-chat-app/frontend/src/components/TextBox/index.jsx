import { useCallback, useMemo, useState, useEffect, useRef } from 'react'
import { Text, Transforms, Editor, createEditor, Range } from 'slate'
import { Slate, Editable, withReact, ReactEditor } from 'slate-react'
import { withHistory } from 'slate-history'
import styled from 'styled-components'
import { Picker } from 'emoji-mart'

import useUsers from '../../hooks/useUsers'
import useNavigation from '../../hooks/useNavigation'
import useAPI from '../../hooks/useAPI'
import Mention from './Mention'
import MentionSuggestion from './MentionSuggestion'
import EmojiQuickSearch from './EmojiQuickSearch'
import EmojiElem from './Emoji'
import Toolbar from './Toolbar'

import emojiSheet from '../../assets/emoji-mart-twitter-images.png'

const SEditable = styled(Editable)`
    width: 100%;
    background: #27282c;
    margin-right: 18px;
    padding-top: 8px;
    padding-bottom: 10px;
    border-radius: 6px;
    padding-left: 48px;
    padding-right: 12px;
    margin-bottom: 8px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    max-height: 300px;
    overflow-y: auto;
    overflow-x: hidden;
    position: relative;
`

const StrikeOut = styled.span`
    text-decoration: line-through;
`

const SCode = styled.code`
    background: #2f3340;
    border-radius: 2px;
    padding-left: 4px;
    padding-right: 4px;
`

const EmojiWrapper = styled.div`
    position: absolute;
    top: -440px;
    right: 20px;
`

const toggleFormat = (editor, format) => {
    const isActive = isFormatActive(editor, format)
    Transforms.setNodes(
        editor,
        { [format]: isActive ? null : true },
        { match: Text.isText, split: true },
    )
}

const isFormatActive = (editor, format) => {
    const [match] = Editor.nodes(editor, {
        match: (n) => n[format] === true,
        mode: 'all',
    })
    return !!match
}

const Leaf = ({ attributes, children: paramChildren, leaf }) => {
    let children = paramChildren
    if (leaf.bold) {
        children = <strong>{children}</strong>
    }

    if (leaf.italic) {
        children = <em>{children}</em>
    }

    if (leaf.underlined) {
        children = <u>{children}</u>
    }

    if (leaf.strike) {
        children = <StrikeOut>{children}</StrikeOut>
    }

    if (leaf.code) {
        children = <SCode>{children}</SCode>
    }

    return <span {...attributes}>{children}</span>
}

const renderElement = (elemProps) => {
    const { element, attributes, children } = elemProps
    switch (element.type) {
        case 'link':
            return (
                <a href={element.url} {...attributes}>
                    {children}
                </a>
            )
        case 'mention':
            return <Mention {...attributes} element={element} />
        case 'emoji':
            return <EmojiElem {...elemProps} />
        default:
            return <div {...attributes}>{children}</div>
    }
}

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

function TextBox(props) {
    const {
        messageText,
        setMessageText,
        initialMessageText,
        attachments,
        editorRef,
        setShowEmojiKeyboard,
        onClickSend,
        controlsRef,
        children,
        showEmojiKeyboard,
        emojiSearchWord,
        setEmojiSearchWord,
        editorFocusPoint,
        setEditorFocusPoint,
    } = props
    // Mention Variables
    const [targetMention, setTargetMention] = useState()
    const [indexMention, setIndexMention] = useState(0)
    const [searchMention, setSearchMention] = useState('')
    const [emojisFound, setEmojisFound] = useState(false)

    const { selectedStateURI } = useNavigation()
    const { users } = useUsers(selectedStateURI)
    const mentionRef = useRef()

    const editor = useMemo(
        () => withEmojis(withMentions(withHistory(withReact(createEditor())))),
        [],
    )

    const userAddresses = useMemo(() => Object.keys(users || {}), [users])
    const mentionUsers = useMemo(
        () =>
            userAddresses
                .map((address) => ({ ...users[address], address }))
                .filter((user) => {
                    if (!user.username && !user.nickname) {
                        return user.address.includes(
                            searchMention.toLowerCase(),
                        )
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
                .slice(0, 10),
        [userAddresses, users, searchMention],
    )

    useEffect(() => {
        editorRef.current = editor
    }, [editor])

    useEffect(() => {
        if (targetMention && mentionUsers.length > 0) {
            const el = mentionRef.current
            const domRange = ReactEditor.toDOMRange(editor, targetMention)
            const rect = domRange.getBoundingClientRect()
            const topCalc = 68 + mentionUsers.length * 28

            el.style.top = `-${topCalc}px`
            el.style.left = '0px'
        }
    }, [mentionUsers, editor, indexMention, searchMention, targetMention])

    const onEditorBlur = useCallback(() => {
        try {
            setEditorFocusPoint(editor.selection.focus)
        } catch (e) {
            console.error(e)
        }
    }, [editor, setEditorFocusPoint])

    const onDOMBeforeInput = useCallback(
        (event) => {
            switch (event.inputType) {
                case 'formatBold':
                    event.preventDefault()
                    return toggleFormat(editor, 'bold')
                case 'formatItalic':
                    event.preventDefault()
                    return toggleFormat(editor, 'italic')
                case 'formatUnderline':
                    event.preventDefault()
                    return toggleFormat(editor, 'underlined')
                case 'formatStrike':
                    event.preventDefault()
                    return toggleFormat(editor, 'strike')
                case 'formatCode':
                    event.preventDefault()
                    return toggleFormat(editor, 'code')
                default:
                    return null
            }
        },
        [editor],
    )

    const insertMention = useCallback(
        (selectedUser) => {
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
        },
        [editor, setMessageText],
    )

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
            cursor -= 1
        }

        return searchWord
    }

    const onChangeMessageText = useCallback(
        (textValue) => {
            setMessageText(textValue)
            const { selection } = editor

            if (selection && Range.isCollapsed(selection)) {
                const [start] = Range.edges(selection)
                const wordBefore = Editor.before(editor, start, {
                    unit: 'word',
                })
                const before = wordBefore && Editor.before(editor, wordBefore)
                const beforeRange =
                    before && Editor.range(editor, before, start)
                const beforeText =
                    beforeRange && Editor.string(editor, beforeRange)
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
                            getCurrentEmojiWord(
                                focus.offset - 1,
                                childNode.text,
                            )
                        }, 0)
                    }
                }
            }
        },
        [editor, setTargetMention, setMessageText],
    )

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

                            insertMention(selectedUser)
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
            emojisFound,
            indexMention,
            targetMention,
            editor,
            emojiSearchWord,
            insertMention,
            mentionUsers,
            onClickSend,
        ],
    )

    return (
        <>
            <Slate
                editor={editor}
                value={messageText}
                onChange={onChangeMessageText}
            >
                <Toolbar
                    toggleFormat={toggleFormat}
                    isFormatActive={isFormatActive}
                />
                <SEditable
                    placeholder="Type message here..."
                    renderLeaf={Leaf}
                    renderElement={renderElement}
                    onKeyDown={onKeyDown}
                    onBlur={onEditorBlur}
                    onDOMBeforeInput={onDOMBeforeInput}
                    spellCheck="false"
                    autoCapitalize="off"
                    autoComplete="off"
                    autoCorrect="off"
                    // NOTE: Implement spell check for Slate
                />
                {children}
                {targetMention && mentionUsers.length > 0 && (
                    <MentionSuggestion
                        mentionUsers={mentionUsers}
                        indexMention={indexMention}
                        mentionRef={mentionRef}
                        controlsRef={controlsRef}
                    />
                )}
            </Slate>
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
        </>
    )
}

export default TextBox

/*
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
    */

/*
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
    */

/*
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
    */
