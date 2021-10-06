import React, { useState, useEffect, Fragment } from 'react'
import { Twemoji } from 'react-emoji-render-redwood'
import styled from 'styled-components'

import Mention from '../TextBox/Mention'

const SNormalizeMessage = styled.div`
    white-space: pre-wrap;
    padding-right: 8px;
    overflow: hidden;
    font-size: 11;
    max-height: 17;
    color: ${(props) =>
        props.selected ? 'rgba(255, 255, 255, .8)' : 'rgba(255, 255, 255, .5)'};
    span.emoji-mart-emoji {
        top: ${(props) => (props.preview ? '2px' : '4px')};
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

function NormalizeMessage({
    msgText,
    preview,
    selected,
    style = {},
    isNotification,
}) {
    const [content, setContent] = useState([])
    useEffect(() => {
        try {
            const msgJson = JSON.parse(msgText)
            const contentLines = msgJson.map((msg) =>
                msg.children.map((msgChild) => {
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
                            decorator = (
                                <span
                                    style={{ textDecoration: 'line-through' }}
                                >
                                    {decorator}
                                </span>
                            )
                        }

                        if (msgChild.code) {
                            decorator = <SCode>{decorator}</SCode>
                        }

                        return decorator
                    }
                    if (msgChild.type === 'emoji') {
                        if (preview) {
                            return (
                                <SEmojiPreview
                                    svg
                                    text={msgChild.value}
                                    options={{
                                        ext: 'svg',
                                        protocol: 'https',
                                        baseUrl:
                                            'twemoji.maxcdn.com/v/12.1.3/svg/',
                                        localSvg: true,
                                    }}
                                />
                            )
                        }

                        return (
                            <SEmoji
                                svg
                                text={msgChild.value}
                                options={{
                                    ext: 'svg',
                                    protocol: 'https',
                                    baseUrl: 'twemoji.maxcdn.com/v/12.1.3/svg/',
                                    localSvg: true,
                                }}
                            />
                        )
                    }
                    if (msgChild.type === 'mention') {
                        return (
                            <Mention
                                element={msgChild}
                                style={{ userSelect: 'auto' }}
                                absolute
                                preview={preview}
                            />
                        )
                    }

                    return null
                }),
            )
            setContent(contentLines)
        } catch (e) {
            setContent([msgText])
        }
    }, [msgText, preview])

    if (preview) {
        return (
            <SNormalizeMessage selected={selected} preview={preview}>
                {content[0]}
            </SNormalizeMessage>
        )
    }

    if (isNotification) {
        return (
            <SNormalizeMessage
                style={{
                    overflow: 'hidden',
                    fontSize: style.fontSize,
                    maxHeight: 50,
                }}
                selected
            >
                {content.map((item, idx) => {
                    if (content.length - 1 === idx) {
                        return <>{item}</>
                    }
                    return (
                        <>
                            {item}
                            <br />
                        </>
                    )
                })}
            </SNormalizeMessage>
        )
    }

    /* eslint-disable react/no-array-index-key */
    return (
        <SNormalizeMessage style={style} selected>
            {content.map((item, idx) => {
                if (content.length - 1 === idx) {
                    return <Fragment key={idx}>{item}</Fragment>
                }
                return (
                    <Fragment key={idx}>
                        {item}
                        <br />
                    </Fragment>
                )
            })}
        </SNormalizeMessage>
    )
    /* eslint-enable react/no-array-index-key */
}

export default NormalizeMessage