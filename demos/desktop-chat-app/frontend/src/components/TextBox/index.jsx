import React, { useCallback } from 'react'
import { Text, Transforms, Editor } from 'slate'
import { Slate, Editable } from 'slate-react'
import styled from 'styled-components'

import Mention from './Mention'
import MentionSuggestion from './MentionSuggestion'
import EmojiElem from './Emoji'
import Toolbar from './Toolbar'

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

    const onDOMBeforeInput = useCallback(
        (event) => {
            switch (event.inputType) {
                case 'formatBold':
                    event.preventDefault()
                    return toggleFormat(props.editor, 'bold')
                case 'formatItalic':
                    event.preventDefault()
                    return toggleFormat(props.editor, 'italic')
                case 'formatUnderline':
                    event.preventDefault()
                    return toggleFormat(props.editor, 'underlined')
                case 'formatStrike':
                    event.preventDefault()
                    return toggleFormat(props.editor, 'strike')
                case 'formatCode':
                    event.preventDefault()
                    return toggleFormat(props.editor, 'code')
                default:
                    return null
            }
        },
        [props.editor],
    )

    return (
        <Slate
            editor={props.editor}
            value={props.value}
            onChange={props.onChange}
        >
            <Toolbar
                toggleFormat={toggleFormat}
                isFormatActive={isFormatActive}
            />
            <SEditable
                placeholder="Type message here..."
                renderLeaf={Leaf}
                renderElement={renderElement}
                onKeyDown={props.onKeyDown}
                onBlur={props.onBlur}
                onDOMBeforeInput={onDOMBeforeInput}
                // NOTE: Implement spell check for Slate
            />
            {props.children}
            {props.targetMention && props.mentionUsers.length > 0 && (
                <MentionSuggestion
                    mentionUsers={props.mentionUsers}
                    indexMention={props.indexMention}
                    mentionRef={props.mentionRef}
                    controlsRef={props.controlsRef}
                />
            )}
        </Slate>
    )
}

export default TextBox
