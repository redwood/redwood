import React, { useState } from 'react'
import styled from 'styled-components'

import ChatBar from './ChatBar'
import ServerBar from './ServerBar'
import UserControl from './UserControl'
import GroupItem from './GroupItem'

const Container = styled.div`
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: #0e1121;
`

const BarContainer = styled.div`
    display: flex;
    height: 100%;
`

const SServerBar = styled(ServerBar)`
    width: 72px;
    height: 100%;
`

const SChatBar = styled(ChatBar)`
    width: 200px;
`

function Sidebar({ selectedStateURI, setSelectedStateURI, className }) {
    const [selectedServer, setSelectedServer] = useState(null)

    return (
        <Container className={className}>
            <UserControl />
            <BarContainer>
                <SServerBar selectedServer={selectedServer} setSelectedServer={setSelectedServer} />
                <SChatBar selectedServer={selectedServer} selectedStateURI={selectedStateURI} setSelectedStateURI={setSelectedStateURI} />
            </BarContainer>
        </Container>
    )
}

export default Sidebar