import { useEffect, useState, useMemo } from 'react'
import styled from 'styled-components'
import { Twemoji } from 'react-emoji-render-redwood'

import emojiData from 'emoji-mart/data/twitter.json'

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
    background: ${(props) => (props.selected ? '#353434' : 'transparent')};
    span {
        &:first-child {
            margin-right: 8px;
        }
    }
`

const SEmoji = styled(Twemoji)`
    color: rgba(255, 255, 255, 1);
    > img {
        vertical-align: -4px !important;
        width: auto !important;
        height: 21px !important;
    }
`

function EmojiQuickSearch(props) {
    const { messageText, setEmojisFound, setEmojiSearchWord, onSelectEmoji } =
        props
    const [selectedEmojiIdx, setSelectedEmojiIdx] = useState(0)

    let top = 68

    const filteredEmojis = useMemo(() => {
        if (messageText.replace(':', '').length >= 2) {
            let count = 0
            const fEmojis = Object.keys(emojiData.emojis).filter((emoji) => {
                let emojiKeywords = emoji
                if (emojiData.emojis[emoji].j) {
                    emojiKeywords = `${emoji}|${emojiData.emojis[emoji].j.join(
                        ',',
                    )}`
                }

                const isMatch = emojiKeywords.includes(
                    messageText.replace(':', ''),
                )

                if (isMatch) {
                    count += 1
                    if (count > 10) {
                        return false
                    }
                    return isMatch
                }
                return false
            })
            setEmojisFound(!!fEmojis.length)
            return fEmojis
        }
        return []
    }, [messageText, setEmojisFound])

    useEffect(
        () => () => {
            setEmojisFound(false)
            setEmojiSearchWord('')
        },
        [setEmojiSearchWord, setEmojisFound],
    )

    useEffect(() => {
        const handleArrowUpDown = (event) => {
            if (
                event.key !== 'ArrowDown' ||
                event.key !== 'ArrowUp' ||
                event.key !== 'Enter'
            ) {
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

            if (event.key === ':') {
                let isMatch = false
                let matchIdx = 0
                filteredEmojis.forEach((emoji, idx) => {
                    if (messageText === emoji.split('|')[0]) {
                        isMatch = true
                        matchIdx = idx
                    }
                })

                if (isMatch) {
                    event.preventDefault()
                    event.stopPropagation()
                    onSelectEmoji(`:${filteredEmojis[matchIdx]}:`)
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
    }, [
        selectedEmojiIdx,
        setSelectedEmojiIdx,
        filteredEmojis,
        messageText,
        onSelectEmoji,
        setEmojisFound,
    ])

    // Handles spacing with different search result counts
    top = 68 + filteredEmojis.length * 30

    if (filteredEmojis.length === 0) {
        top += 20
    }

    return (
        <EmojiContainer style={{ top: `-${top}px` }}>
            <EmojiHeader>
                Emoji Search <b>{props.messageText}</b>
            </EmojiHeader>
            {filteredEmojis.length === 0 ? <div>No emojis found.</div> : null}
            {filteredEmojis.map((emoji, idx) => (
                <EmojiItemWrapper
                    selected={selectedEmojiIdx === idx}
                    key={emoji}
                >
                    <SEmoji
                        svg
                        text={`:${emoji}:`}
                        options={{
                            protocol: 'https',
                            baseUrl: 'twemoji.maxcdn.com/v/12.1.3/svg/',
                            ext: 'svg',
                            localSvg: true,
                        }}
                    />
                    {emoji}
                </EmojiItemWrapper>
            ))}
        </EmojiContainer>
    )
}

export default EmojiQuickSearch
