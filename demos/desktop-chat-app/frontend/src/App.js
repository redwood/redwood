import React, { useState } from 'react'
import styled from 'styled-components'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import StateTreeDebugView from './components/StateTreeDebugView'

const Layout = styled.div`
    display: flex;
    height: 100vh;
    font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
`

const SSidebar = styled(Sidebar)`
    width: 150px;
    background-color: #252525;
    color: white;
`

const SChat = styled(Chat)`
    flex-grow: 1;
`

const SStateTreeDebugView = styled(StateTreeDebugView)`
    width: 600px;
`

function App() {
    const [selectedStateURI, setSelectedStateURI] = useState(null)

    return (
        <Layout>
            <SSidebar selectedStateURI={selectedStateURI} setSelectedStateURI={setSelectedStateURI} />
            <SChat stateURI={selectedStateURI} />
            <SStateTreeDebugView/>
        </Layout>
    )
}

export default App
