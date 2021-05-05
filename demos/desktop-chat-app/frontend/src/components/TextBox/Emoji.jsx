import React from 'react'
import styled from 'styled-components'

import { Emoji as EmojiMart } from 'emoji-mart'

const SEmojiWrapper = styled.span`
  vertical-align: baseline;
  display: inline-block;
  border-radius: 4px;
  .emoji-mart-emoji {
    top: 4px;
  }
`

function Emoji({ attributes, children, element }) {
  return (
    <SEmojiWrapper
      {...attributes}
      contentEditable={false}
    >
      <EmojiMart emoji={element.value.replace(':', '').replace(':', '')} size={18} />
      {children}
    </SEmojiWrapper>
  )
}

export default Emoji