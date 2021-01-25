import React, { useState } from 'react'
import styled from 'styled-components'

import ChatBar from './ChatBar'
import ServerBar from './ServerBar'
import UserControl from './UserControl'
import GroupItem from './GroupItem'

const serverBarVerticalPadding = '12px'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: ${props => props.theme.color.grey[600]};
`

const BarContainer = styled.div`
    display: flex;
    height: 100%;
`

const SServerBar = styled(ServerBar)`
    width: 72px;
    height: calc(100% - 2 * ${props => serverBarVerticalPadding});
`

const SChatBar = styled(ChatBar)`
    width: 250px;
    flex-grow: 1;
`

const ChatAndUserWrapper = styled.div`
    display: flex;
    flex-direction: column;
`

function Sidebar({ selectedStateURI, setSelectedStateURI, className }) {
    const [selectedServer, setSelectedServer] = useState(null)

    return (
        <Container className={className}>
            <BarContainer>
                <SServerBar selectedServer={selectedServer} setSelectedServer={setSelectedServer} verticalPadding={serverBarVerticalPadding} />
                <ChatAndUserWrapper>
                    <SChatBar selectedServer={selectedServer} selectedStateURI={selectedStateURI} setSelectedStateURI={setSelectedStateURI} />
                    <UserControl />
                </ChatAndUserWrapper>
            </BarContainer>
        </Container>
    )
}

export default Sidebar