import React, { useMemo, useCallback } from 'react'
import { Text, Transforms, Editor } from 'slate'
import { Slate, Editable } from 'slate-react'
import styled from 'styled-components'
import { Emoji } from 'emoji-mart'

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
  border: 1px solid rgba(255,255,255, .18);
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
    { match: Text.isText, split: true }
  )
}

const isFormatActive = (editor, format) => {
  const [match] = Editor.nodes(editor, {
    match: n => n[format] === true,
    mode: 'all',
  })
  return !!match
}

const Leaf = ({ attributes, children, leaf }) => { 
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

function TextBox(props) {
  const onKeyDown = useCallback((event) => {
    if (targetMention && mentionUsers.length > 0) {
      switch (event.key) {
      case 'ArrowDown':
          event.preventDefault()
          const prevIndex = indexMention >= mentionUsers.length - 1 ? 0 : indexMention + 1
          setIndexMention(prevIndex)
          break
      case 'ArrowUp':
          event.preventDefault()
          const nextIndex = indexMention <= 0 ? mentionUsers.length - 1 : indexMention - 1
          setIndexMention(nextIndex)
          break
      case 'Tab':
      case 'Enter':
          const selectedUser = mentionUsers[indexMention]
          if (selectedUser) {
            event.preventDefault()

            Transforms.select(editor, targetMention)

            insertMention(editor, selectedUser)
            setTargetMention(null)
          }
          break
      case 'Escape':
          event.preventDefault()
          setTargetMention(null)
          break
      }
    } else {
      // Emoji Handling
      if (event.code === 'Enter' && !event.shiftKey) {
        if (!emojisFound || (!emojisFound && emojiSearchWord)) {
          event.preventDefault()
          event.stopPropagation()

          onClickSend()
        }
      }
    }

    if (emojiSearchWord) {
      if (event.code === 'ArrowUp' || event.code === 'ArrowDown') {
        event.preventDefault()
      }
    }
  }, [messageText, emojisFound, indexMention, searchMention, targetMention])


  const renderElement = useCallback((props) => {
    const { element, attributes, children } = props
    switch (element.type) {
      case 'link':
        return <a href={element.url} {...attributes}>{children}</a>
      case 'mention':
        return <Mention {...props} />
      case 'emoji':
        return <EmojiElem {...props} />
      default:
        return <div {...attributes}>{children}</div>
    }
  }, [])

  return (
    <Slate
      editor={props.editor}
      value={props.value}
      onChange={props.onChange}
    >
      <Toolbar toggleFormat={toggleFormat} isFormatActive={isFormatActive} />
      <SEditable
        placeholder={'Type message here...'}
        renderLeaf={props => <Leaf {...props} />}
        renderElement={renderElement}
        onKeyDown={props.onKeyDown}
        onBlur={props.onBlur}
        onDOMBeforeInput={(event) => {
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
          }
        }}
        // spellCheck
      />
      {props.children}
      {props.targetMention && props.mentionUsers.length > 0 && (
        <MentionSuggestion
          mentionUsers={props.mentionUsers}
          indexMention={props.indexMention}
          controlsRef={props.controlsRef}
        />
      )}
    </Slate>
  )
}

export default TextBox