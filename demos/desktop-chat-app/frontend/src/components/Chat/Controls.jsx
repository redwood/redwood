

const ControlsContainer = styled.div`
    display: flex;
    align-self: end;
    padding-bottom: 6px;
    width: 100%;
    margin-top: 6px;
    position: relative;
`

const TextBoxButtonWrapper = styled.div`
  position: absolute;
  right: 20px;
  bottom: 18px;
  display: flex;
  align-items: center;
  justify-content;
  height: 32px;
`

const EmojiWrapper = styled.div`
  position: absolute;
  top: -440px;
  right: 20px;
`

const withMentions = editor => {
    const { isInline, isVoid } = editor
    editor.isInline = (element) => element.type === 'mention' ? true : isInline(element)
    editor.isVoid   = (element) => element.type === 'mention' ? true : isVoid(element)
    return editor
}

const withEmojis = editor => {
    const { isInline, isVoid } = editor
    editor.isInline = (element) => element.type === 'emoji' ? true : isInline(element)
    editor.isVoid   = (element) => element.type === 'emoji' ? true : isVoid(element)
    return editor
}

function Controls({ attachmentsInputRef, attachmentFormRef }) {
    const theme = useTheme()
    const controlsRef = useRef()

    const initialMessageText = [{
        type: 'paragraph',
        children: [{ text: '' }],
    }]
    const [messageText, setMessageText] = useState(initialMessageText)

    // Init Slate Editor
    // const editor = useMemo(() => withMentions(withHistory(withReact(createEditor()))), [])
    const editorRef = useRef()
    if (!editorRef.current) editorRef.current = withEmojis(withMentions(withHistory(withReact(createEditor()))))
    const editor = editorRef.current

    const insertMention = useCallback((editor, selectedUser) => {
        const mention = {
            type: 'mention',
            selectedUser,
            children: [{ text: '' }],
        }
        Transforms.insertNodes(editor, [mention, {
            text: ' '
        }])
        Transforms.move(editor, { distance: 2 })
        setMessageText(editor.children)
    }, [setMessageText])


    const onClickAddAttachment = useCallback(() => {
        if (previews.length === 0) {
            attachmentsInputRef.current.value === ''
            attachmentFormRef.current.reset()
        }
        attachmentsInputRef.current.click()
    }, [previews])

    const serializeMessageText = useCallback(() => {
      let isEmpty = true
      const filteredMessage = []
      messageText.forEach((node) => {
        if (node.children.length === 1) {
          let trimmedNode = {}
          if (typeof node.children[0].text === 'string') {
            trimmedNode = {
              ...node,
              children: [{
                ...node.children[0],
                text: node.children[0].text.trim(),
              }]
            }
          }

          if (JSON.stringify(initialMessageText[0]) === JSON.stringify(trimmedNode)) {
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

    // Emoji Variables
    const [showEmojiKeyboard, setShowEmojiKeyboard] = useState(false)
    const [emojiSearchWord, setEmojiSearchWord] = useState('')
    const [emojisFound, setEmojisFound] = useState(false)

    const onOpenEmojis = useCallback((event) => {
      if (showEmojiKeyboard) {
        setShowEmojiKeyboard(!showEmojiKeyboard)
      } else {
        setShowEmojiKeyboard(!showEmojiKeyboard)
      }
    }, [setShowEmojiKeyboard, showEmojiKeyboard])

    const onSelectEmoji = (emoji) => {
      if (typeof emoji === 'string') {
        const [start] = Range.edges(editor.selection)
        const wordBefore = Editor.before(editor, start, { unit: 'word' })
        const before = wordBefore && Editor.before(editor, wordBefore)
        const beforeRange = before && Editor.range(editor, before, start)
        Transforms.select(editor, {
          anchor: {
            path: beforeRange.anchor.path,
            offset: beforeRange.focus.offset - emojiSearchWord.length - 1
          },
          focus: beforeRange.focus,
        })

        const emojiNode = {
          type: 'emoji',
          value: emoji,
          children: [{ text: '' }],
        }
        Transforms.insertNodes(editor, [emojiNode, {
          text: ' '
        }])
        Transforms.move(editor, { distance: 2 })
      } else {
        // Set focus point from blurred
        editor.selection = { anchor: editorFocusPoint, focus: editorFocusPoint }
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

    const onClickSend = useCallback(async () => {
        const plainMessage = serializeMessageText()
        if (!api || (!plainMessage && attachments.length === 0)) { return }
        // Replace with markdown serializer
        await api.sendMessage(plainMessage, attachments, nodeIdentities[0].address, selectedServer, selectedRoom, messages)
        setAttachments([])
        setPreviews([])
        setEmojiSearchWord('')

        // Reset SlateJS cursor
        const point = { path: [0, 0], offset: 0 };
        setEditorFocusPoint(point)
        editor.selection = { anchor: point, focus: point };
        editor.history = { redos: [], undos: [] };

        setMessageText(initialMessageText)

        attachmentsInputRef.current.value = ''
    }, [messageText, nodeIdentities, attachments, selectedServer, selectedRoom, messages, api, previews])

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
          if (anchor.offset === focus.offset && JSON.stringify(anchor.path) === JSON.stringify(focus.path)) {
            const parentNode = editor.children[focus.path[0]]
            const childNode = parentNode.children[focus.path[1]]

            if (childNode.text) {
              setTimeout(() => {
                getCurrentEmojiWord(focus.offset - 1, childNode.text)
              }, 0);
            }
          }
        }
    }

    return (
        <ControlsContainer ref={controlsRef}>
            <AddAttachmentButton onClick={onClickAddAttachment} style={{ color: theme.color.white }} />
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
                controlsRef={controlsRef}
            >
              <TextBoxButtonWrapper>
                <SIconButton onClick={onOpenEmojis}><EmojiEmotionsIcon /></SIconButton>
                <SIconButton onClick={onClickSend}><SendIcon /></SIconButton>
              </TextBoxButtonWrapper>
            </TextBox>
            { showEmojiKeyboard ? <EmojiWrapper>
              <Picker
                useButton={false}
                title={'Hush Chat'}
                backgroundImageFn={(set, sheetSize) => {
                    return emojiSheet
                }}
                perLine={8}
                set='twitter'
                theme='dark'
                emojiSize={24}
                onSelect={onSelectEmoji}
              />
            </EmojiWrapper> : null }
            { emojiSearchWord ? <EmojiQuickSearch
              emojisFound={emojisFound}
              setEmojisFound={setEmojisFound}
              setEmojiSearchWord={setEmojiSearchWord}
              onSelectEmoji={onSelectEmoji}
              messageText={emojiSearchWord}
            /> : null }
        </ControlsContainer>
    )
}
