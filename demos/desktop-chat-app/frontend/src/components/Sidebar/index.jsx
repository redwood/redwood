import React from 'react'
import styled from 'styled-components'

import ChatBar from './ChatBar'
import UserControl from './UserControl'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    height: 100%;
`

const BarContainer = styled.div`
    display: flex;
    height: 100%;
`

const SChatBar = styled(ChatBar)`
    width: ${(props) => props.theme.chatSidebarWidth};
    flex-grow: 1;
`

const ChatAndUserWrapper = styled.div`
    display: flex;
    flex-direction: column;
`

function Sidebar({ className }) {
    return (
        <Container className={className}>
            <BarContainer>
                <ChatAndUserWrapper>
                    <SChatBar />
                    <UserControl />
                </ChatAndUserWrapper>
            </BarContainer>
        </Container>
    )
}

export default Sidebar
