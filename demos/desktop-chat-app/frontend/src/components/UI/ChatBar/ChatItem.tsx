import { CSSProperties } from 'react'
import styled, { css } from 'styled-components'

import P from '../Text/P'
import Header from '../Text/Header'
import ChatIcon from './ChatIcon'

interface ChatItemProps {
    style?: CSSProperties
    chatName: string
    preview?: string
    selected?: boolean
    onClick?: () => unknown
}

const SChatItem = styled.div<{ selected?: boolean }>`
    padding: 8px;
    width: 100%;
    border-radius: 4px;
    background: transparent;
    display: flex;
    box-sizing: border-box;
    transition: ${({ theme }) => theme.transition.primary};
    cursor: pointer;
    &:hover {
        background: ${({ theme }) => theme.color.chatHovered};
    }
    ${({ selected, theme }) =>
        selected &&
        css`
            background: ${theme.color.chatSelected} !important;
        `}
`
const SStackInfo = styled.div`
    display: flex;
    flex-direction: column;
    width: 180px;
    padding-left: 8px;
`

const SSubHeader = styled(Header)`
    font-size: 16px;
    margin: 0px;
    overflow-x: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin-bottom: 2px;
`

const SP = styled(P)`
    font-size: 10px;
    margin: 0px;
    padding: 0px;
    font-weight: 300;
    overflow-x: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
`

function ChatItem({
    style = {},
    chatName = '',
    preview = '',
    selected = false,
    onClick = () => false,
}: ChatItemProps): JSX.Element {
    return (
        <SChatItem onClick={onClick} selected={selected} style={style}>
            <ChatIcon chatName={chatName} />
            <SStackInfo>
                <SSubHeader>{chatName}</SSubHeader>
                <SP>{preview}</SP>
            </SStackInfo>
        </SChatItem>
    )
}

export default ChatItem
