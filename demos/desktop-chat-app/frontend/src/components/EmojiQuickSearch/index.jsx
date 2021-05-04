import React, { useEffect, useState } from 'react'
import styled from 'styled-components'
import { Emoji } from 'emoji-mart'
import emojiData from 'emoji-mart/data/apple.json'

const EmojiContainer = styled.div`
  width: calc(100% - 36px);
  background: #222222;
  position: absolute;
  top: -118px;
  border-radius: 6px;
  border: 1px solid #555453;
  padding: 8px;
`

const EmojiHeader = styled.div`
  width: 100%;
  border-bottom: 1px solid #555453;
  padding-bottom: 4px;
  margin-bottom: 6px;
`

const EmojiItemWrapper = styled.div`
  font-size: 18px;
  display: flex;
  align-items: center;
  padding-top: 2px;
  padding-bottom: 2px;
  padding-left: 8px;
  font-weight: bold;
  border-radius: 4px;
  background: ${(props) => props.selected ? '#353434' : 'transparent'};
  span {
    &:first-child {
      margin-right: 4px;
    }
  }
`

function EmojiQuickSearch(props) {
  const { messageText, setEmojisFound, setEmojiSearchWord, onSelectEmoji } = props
  const [selectedEmojiIdx, setSelectedEmojiIdx] = useState(0)

  let top = 68
  let filteredEmojis = []

  useEffect(() => {
    return () => {
      setEmojisFound(false)
      setEmojiSearchWord('')
    }
  }, [])

  useEffect(() => {
    const handleArrowUpDown = (event) => {
      setEmojisFound(!!filteredEmojis.length)
      
      if (event.key !== 'ArrowDown' || event.key !== 'ArrowUp' || event.key !== 'Enter') {
        setSelectedEmojiIdx(0)
      }

      if (event.key === 'ArrowDown') {
        event.preventDefault()
        event.stopPropagation()
        if (filteredEmojis.length - 1 > selectedEmojiIdx) {
          setSelectedEmojiIdx(selectedEmojiIdx + 1)
        } else {
          setSelectedEmojiIdx(0)
        }
      }

      if (event.key === 'ArrowUp') {
        event.preventDefault()
        event.stopPropagation()
        if (selectedEmojiIdx === 0) {
          setSelectedEmojiIdx(filteredEmojis.length - 1)
        } else {
          setSelectedEmojiIdx(selectedEmojiIdx - 1)
        }
      }

      if (event.key === 'Enter' && !event.shiftKey) {
        if (filteredEmojis.length > 0) {
          event.preventDefault()
          event.stopPropagation()
          setEmojisFound(false)
          onSelectEmoji(`:${filteredEmojis[selectedEmojiIdx]}:`)
        }
      }
    }
  
    window.addEventListener('keydown', handleArrowUpDown)

    return () => {
      window.removeEventListener('keydown', handleArrowUpDown)
    }
  }, [selectedEmojiIdx, setSelectedEmojiIdx, filteredEmojis])

  const filterEmojis = () => {
    let count = 0
    return Object.keys(emojiData.emojis).filter((emoji) => {
      let emojiKeywords = emoji
      if (emojiData.emojis[emoji].j) {
        emojiKeywords = `${emoji}|${emojiData.emojis[emoji].j.join(',')}`
      }

      const isMatch = emojiKeywords.includes(messageText.replace(':', ''))

      if (isMatch) {
        count++
        if (count > 10) {
          return
        }
        return isMatch
      }
    })
  }

  if (messageText.replace(':', '').length >= 2) {
    filteredEmojis = filterEmojis()
  }

  // Handles spacing with different search result counts
  top = 68 + (filteredEmojis.length * 30)
  
  if (filteredEmojis.length === 0) {
    top += 20
  }

  return (
    <EmojiContainer style={{ top: `-${top}px` }}>
      <EmojiHeader>Emoji Search <b>{props.messageText}</b></EmojiHeader>
      {
        filteredEmojis.length === 0 ? <div>No emojis found.</div> : null
      }
      {
        filteredEmojis.map((emoji, idx) => {
          return <EmojiItemWrapper selected={selectedEmojiIdx === idx} key={idx}>
            <Emoji emoji={emoji} size={18} /> 
            {emoji}
               
          </EmojiItemWrapper>
        })
      }
    </EmojiContainer>
  )
}

export default EmojiQuickSearch