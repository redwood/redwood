import { CSSProperties, useState } from 'react'
import styled from 'styled-components'
import ContactsIcon from '@material-ui/icons/PeopleAlt'

import IconWrapper from '../Icon/Wrapper'
import Header from '../Text/Header'
import ChatItem from './ChatItem'

interface ChatBarProps {
    style?: CSSProperties
    stateURI?: string
}

const SChatBar = styled.div`
    display: flex;
    flex-direction: column;
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
    align-items: center;
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

function ChatBar({ style = {}, stateURI = '' }: ChatBarProps): JSX.Element {
    const [selectedChat, setSelectedChat] = useState('')

    return (
        <SChatBar style={style}>
            <ChatBarHeader stateURI={stateURI} />
            <SChatItemContainer>
                <ChatItem
                    chatName="General"
                    preview="Testing something that is right here"
                    onClick={() => setSelectedChat('General')}
                    selected={selectedChat === 'General'}
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
        </SChatBar>
    )
}

export default ChatBar
