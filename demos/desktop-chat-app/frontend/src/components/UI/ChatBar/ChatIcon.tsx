import { useMemo } from 'react'
import styled from 'styled-components'
import { createAvatar } from '@dicebear/avatars'
import * as style from '@dicebear/avatars-identicon-sprites'
import { desaturate, darken } from 'polished'

import strToColor from '../../../utils/strToColor'

const generateBgColor = (text: string): string =>
    desaturate(0.25, darken(0.1, strToColor(text)))

interface ChatIconProps {
    chatName: string
}

const SChatIcon = styled.img`
    height: 24px;
`

const SChatIconWrapper = styled.div<{ bgColor: string }>`
    background: ${({ bgColor }) => bgColor};
    height: 36px;
    width: 36px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
`

function ChatIcon({ chatName }: ChatIconProps): JSX.Element {
    const chatIcon = useMemo(
        () =>
            createAvatar(style, {
                seed: chatName,
                base64: true,
            }),
        [chatName],
    )
    const bgColor = useMemo(() => generateBgColor(chatName), [chatName])

    return (
        <SChatIconWrapper bgColor={bgColor}>
            <SChatIcon alt={chatName} src={chatIcon} />
        </SChatIconWrapper>
    )
}

export default ChatIcon
