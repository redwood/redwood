import React, { useState, useEffect, Fragment } from 'react'
import { Emoji } from 'emoji-mart'
import { Twemoji } from 'react-emoji-render-redwood'
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

const SCode = styled.code`
  background: #2f3340;
  border-radius: 2px;
  padding-left: 4px;
  padding-right: 4px;
`

const SEmoji = styled(Twemoji)`
  color: rgba(255, 255, 255, 1);
  > img {
    vertical-align: -4px !important;
    width: auto !important;
    height: 21px !important;
  }
`

const SEmojiPreview = styled(Twemoji)`
  color: rgba(255, 255, 255, 1);
  > img {
    vertical-align: -2px !important;
    width: auto !important;
    height: 14px !important;
  }
`


function NormalizeMessage({ msgText, preview, selected, style = {}, isNotification }) {
  const [content, setContent] = useState([])
  useEffect(() => {
    try {
      const msgJson = JSON.parse(msgText)
      const contentLines = msgJson.map((msg) => {
        return msg.children.map((msgChild) => {
          if (msgChild.text !== undefined) {
            let decorator = msgChild.text

            if (msgChild.bold) {
              decorator = <b>{decorator}</b>
            }
            if (msgChild.italic) {
              decorator = <em>{decorator}</em>
            }

            if (msgChild.underlined) {
              decorator = <u>{decorator}</u>
            }

            if (msgChild.strike) {
              decorator = <span style={{ textDecoration: 'line-through'}}>{decorator}</span>
            }

            if (msgChild.code) {
              decorator = <SCode>{decorator}</SCode>
            }

            return decorator
          } else if (msgChild.type === 'emoji') {
            if (preview) {
              return <SEmojiPreview svg text={msgChild.value} />
            }

            return <SEmoji svg text={msgChild.value} options={{
              protocol: 'https',
              baseUrl: 'twemoji.maxcdn.com/v/12.1.3/svg/',
              ext: 'svg'
            }} />

            // if (msgChild.value === ':smiley:') {
            //   return <Emoji backgroundImageFn={() => emojiSheet} emoji={msgChild.value.replace(':', '').replace(':', '')} size={preview ? 14 : 21} />
            // }
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

  if (isNotification) {
	return <SNormalizeMessage style={{
		overflow: 'hidden',
		fontSize: style.fontSize,
		maxHeight: 50,
	}} selected={true}>{content.map((item, idx) => {
		if ((content.length - 1) === idx) {
		  return <Fragment key={idx + item}>{item}</Fragment>
		}
		return <Fragment key={idx + item}>{item}<br /></Fragment>
	  })}</SNormalizeMessage>
  }

  return <SNormalizeMessage style={style} selected={true}>{content.map((item, idx) => {
    if ((content.length - 1) === idx) {
      return <Fragment key={idx + item}>{item}</Fragment>
    }
    return <Fragment key={idx + item}>{item}<br /></Fragment>
  })}</SNormalizeMessage>
}

export default NormalizeMessage