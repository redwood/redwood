import { CSSProperties, useState, useCallback, MouseEvent } from 'react'
import styled from 'styled-components'
import ContactsIcon from '@material-ui/icons/PeopleAlt'
import SettingsIcon from '@material-ui/icons/Settings'
import { Tooltip, ButtonBase } from '@material-ui/core'

import IconWrapper from '../Icon/Wrapper'
import Header from '../Text/Header'
import ChatItem from './ChatItem'
import UserAvatar from '../UserAvatar'

interface ChatBarProps {
    style?: CSSProperties
    stateURI?: string
    nodeIdentity?: string
    username?: string
    avatarImage?: string
}

const SChatBar = styled.div`
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    width: 260px;
    min-height: 600px;
    box-shadow: rgb(0 0 0 / 20%) 0px 2px 1px -1px,
        rgb(0 0 0 / 14%) 0px 1px 1px 0px, rgb(0 0 0 / 12%) 0px 1px 3px 0px;
    background: ${({ theme }) => theme.color.secondaryBackground};
    color: ${({ theme }) => theme.color.text};
`

const SChatBarHeader = styled.div`
    color: ${({ theme }) => theme.color.text};
    padding: 8px 0px;
    margin-left: 8px;
    margin-right: 8px;
    border-bottom: 1px solid ${({ theme }) => theme.color.primary};
    display: flex;
    justify-content: space-between;
    align-items: space-between;
`

const SHeader = styled(Header)`
    font-size: 20px;
    margin: 0px;
    align-items: center;
`

const SRightHeader = styled.div`
    height: 100%;
`

const SIcon = styled(IconWrapper)`
    display: flex;
    align-items: center;
    justify-content: center;
    transition: ${({ theme }) => theme.transition.primary};
    cursor: pointer;
    &:hover {
        transform: scale(1.1);
    }
`

const SChatItemContainer = styled.div`
    display: flex;
    flex-direction: column;
    margin: 8px;
    > div {
        margin-bottom: 8px;
    }
`

function ChatBarHeader({ stateURI = '' }: { stateURI: string }) {
    return (
        <SChatBarHeader>
            <SHeader>{stateURI}</SHeader>
            <SRightHeader>
                <SIcon size={20} icon={<ContactsIcon />} />
            </SRightHeader>
        </SChatBarHeader>
    )
}

const SChatBarUserControls = styled(ButtonBase)`
    &&& {
        display: flex;
        box-sizing: border-box;
        justify-content: space-between;
        align-items: center;
        padding: 8px 0px;
        margin-left: 8px;
        margin-right: 8px;
        width: calc(100% - 16px);
        border-top: 1px solid ${({ theme }) => theme.color.accent1};
        transition: ${({ theme }) => theme.transition.primary};
        cursor: pointer;
        &:hover {
            width: 100%;
            margin-left: 0px;
            margin-right: 0px;
            padding-left: 8px;
            padding-right: 8px;
            border-color: ${({ theme }) => theme.color.primary};
            background: ${({ theme }) => theme.color.chatHovered};
        }
        .MuiTooltip-tooltip {
            background: black !important;
            color: red !important;
        }
    }
`

const SUControlsLeft = styled.div`
    display: flex;
    align-items: center;
`

const SUControlsRight = styled.div``

const SUControlsInfo = styled.div`
    display: flex;
    flex-direction: column;
    width: 120px;
    span {
        font-size: 16px;
        font-family: ${({ theme }) => theme.font.type.secondary};
        overflow-x: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        padding-bottom: 2px;
        text-align: left;
        &:last-child {
            font-size: 12px;
            font-family: ${({ theme }) => theme.font.type.primary};
            overflow-x: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            padding-bottom: 0px;
        }
    }
`

const STooltip = styled(Tooltip)``

const copyKey = (
    addr: string,
    e: MouseEvent<HTMLDivElement>,
    cb: () => unknown,
) => {
    e.stopPropagation()
    e.preventDefault()
    navigator.clipboard.writeText(addr)
    setTimeout(() => {
        cb()
    }, 2000)
}

function ChatBarUserControls({
    nodeIdentity = '',
    username = '',
    avatarImage = '',
}: {
    nodeIdentity: string
    username: string
    avatarImage: string
}) {
    const [copyText, setCopyText] = useState('Click to Copy Address')
    const memoCopyKey = useCallback(
        (e) => {
            setCopyText('Address Copied!')
            copyKey(nodeIdentity, e, () => {
                setCopyText('Click to Copy Address')
            })
        },
        [nodeIdentity, setCopyText],
    )

    return (
        <SChatBarUserControls>
            <SUControlsLeft>
                <UserAvatar
                    address={nodeIdentity}
                    username={username}
                    imageSrc={avatarImage}
                />
                <SUControlsInfo>
                    <span>{username || 'Public Key:'}</span>
                    <STooltip
                        title={copyText}
                        arrow
                        placement="top"
                        onClick={memoCopyKey}
                    >
                        <span>{nodeIdentity}</span>
                    </STooltip>
                </SUControlsInfo>
            </SUControlsLeft>
            <SUControlsRight>
                <SIcon size={20} icon={<SettingsIcon />} />
            </SUControlsRight>
        </SChatBarUserControls>
    )
}

function ChatBar({
    style = {},
    stateURI = '',
    nodeIdentity = '',
    username = '',
    avatarImage = '',
}: ChatBarProps): JSX.Element {
    const [selectedChat, setSelectedChat] = useState('')

    return (
        <SChatBar style={style}>
            <div>
                <ChatBarHeader stateURI={stateURI} />
                <SChatItemContainer>
                    <ChatItem
                        chatName="Default"
                        preview="Testing something that is right here"
                        onClick={() => setSelectedChat('Default')}
                        selected={selectedChat === 'Default'}
                    />
                    <ChatItem
                        chatName="Dank Memes"
                        preview="OMG Man that is a cool meme I was laughing so much at it"
                        onClick={() => setSelectedChat('Dank Memes')}
                        selected={selectedChat === 'Dank Memes'}
                    />
                    <ChatItem
                        chatName="GitHub Issues"
                        preview="Guys I found a major bug in Austin's code. That guy is a really bad programmer"
                        onClick={() => setSelectedChat('Github Issues')}
                        selected={selectedChat === 'Github Issues'}
                    />
                    <ChatItem
                        chatName="Ghost Stories"
                        preview="A ghost visisted me once night in Switzerland."
                        onClick={() => setSelectedChat('Ghost Stories')}
                        selected={selectedChat === 'Ghost Stories'}
                    />
                </SChatItemContainer>
            </div>
            <ChatBarUserControls
                nodeIdentity={nodeIdentity}
                username={username}
                avatarImage={avatarImage}
            />
        </SChatBar>
    )
}

export default ChatBar
