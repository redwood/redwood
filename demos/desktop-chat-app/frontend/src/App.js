import React, { useState } from 'react'
import styled from 'styled-components'

import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import StateTreeDebugView from './components/StateTreeDebugView'

const Layout = styled.div`
    display: flex;
    height: 100vh;
    font-family: 'Noto Sans KR';
    font-weight: 300;
    color: ${props => props.theme.color.white};
`

const SChat = styled(Chat)`
    flex-grow: 1;
    padding-left: 16px;
`

const SStateTreeDebugView = styled(StateTreeDebugView)`
    width: 600px;
`

function App() {
    return (
        <Layout>
            <Sidebar />
            <SChat />
            <SStateTreeDebugView />
        </Layout>
    )
}

export default App
