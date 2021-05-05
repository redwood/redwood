import React, { useMemo, useCallback } from 'react'
import { Slate, Editable, withReact } from 'slate-react'
import styled from 'styled-components'
import { Emoji } from 'emoji-mart'

import Mention from './Mention'
import MentionSuggestion from './MentionSuggestion'
import EmojiElem from './Emoji'


const STextBox = styled.div`
  padding-left: 34px;
  font-family: 'Noto Sans KR';
  font-size: 14px;
`

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

function TextBox(props) {
  const renderElement = useCallback((props) => {
    const { element, attributes, children } = props
    switch (element.type) {
      case 'code':
        return <pre {...attributes}>{children}</pre>
      case 'image':
        return <span {...attributes}>
          <span contentEditable={false}>
            <Emoji emoji={'smiley'} size={21} />
          </span>
          {children}
        </span>
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

  const renderLeaf = useCallback(({ attributes, children, leaf }) => {
    return (
      <span
        {...attributes}
        style={{
          fontWeight: leaf.bold ? 'bold' : 'normal',
          fontStyle: leaf.italic ? 'italic' : 'normal',
        }}
      >
        {children}
      </span>
    )
  }, [])

  return (
    <Slate
      editor={props.editor}
      value={props.value}
      onChange={props.onChange}
    >
      <SEditable
        placeholder={'Type message here...'}
        renderLeaf={renderLeaf}
        renderElement={renderElement}
        onKeyDown={props.onKeyDown}
        onBlur={props.onBlur}
        // spellCheck
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