import React from 'react'
import styled from 'styled-components'

import { Twemoji } from 'react-emoji-render-redwood'

const SEmojiWrapper = styled.span`
  vertical-align: baseline;
  display: inline-block;
  border-radius: 4px;
  .emoji-mart-emoji {
    top: 4px;
  }
`

const SEmojiPreview = styled(Twemoji)`
  color: rgba(255, 255, 255, 1);
  > img {
    vertical-align: -3px !important;
    width: auto !important;
    height: 18px !important;
  }
`

function Emoji({ attributes, children, element }) {
  return (
    <SEmojiWrapper
      {...attributes}
      contentEditable={false}
    >
      <SEmojiPreview
        text={element.value}
      />
      {children}
    </SEmojiWrapper>
  )
}

export default Emoji