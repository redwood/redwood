import React, { useState, useEffect, Fragment } from 'react'
import { Emoji } from 'emoji-mart'
import styled from 'styled-components'

import Mention from './../TextBox/Mention'

const SNormalizeMessage = styled.div`
  white-space: pre-wrap;
  padding-right: 8px;
  color: ${props => props.selected ? 'rgba(255, 255, 255, .8)' : 'rgba(255, 255, 255, .5)'};
  span.emoji-mart-emoji {
    top: ${props => props.preview ? '2px' : '4px'};
  }
` 


function NormalizeMessage({ msgText, preview, selected }) {
  const [content, setContent] = useState([])
  useEffect(() => {
    try {
      const msgJson = JSON.parse(msgText)
      const contentLines = msgJson.map((msg) => {
        return msg.children.map((msgChild) => {
          if (msgChild.text !== undefined) {
            return msgChild.text
          } else if (msgChild.type === 'emoji') {
            return <Emoji emoji={msgChild.value.replace(':', '').replace(':', '')} size={preview ? 14 : 21} />
          } else if (msgChild.type === 'mention') {
            return <Mention element={msgChild} style={{ userSelect: 'auto' }} absolute preview={preview} />        
          }
        })
      })
      setContent(contentLines)
    } catch (e) {
      console.log('Could not render message. Defaulting to text')
      setContent([ msgText ])
    }
  }, [msgText])

  if (preview) {
    return <SNormalizeMessage selected={selected} style={{ overflow: 'hidden', fontSize: 11, maxHeight: 17 }} preview={preview}>{content[0]}</SNormalizeMessage>
  }

  return <SNormalizeMessage selected={true}>{content.map((item, idx) => {
    if ((content.length - 1) === idx) {
      return <Fragment>{item}</Fragment> 
    }
    return <Fragment>{item}<br /></Fragment>
  })}</SNormalizeMessage>
}

export default NormalizeMessage