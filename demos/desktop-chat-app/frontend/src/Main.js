import React, { useState, useRef, useEffect, useCallback } from 'react'
import styled from 'styled-components'
import { Redirect, useHistory } from 'react-router-dom'
import { Code as CodeIcon } from '@material-ui/icons'

import { useRedwood } from 'redwood-p2p-client/react'
import ServerBar from './components/Sidebar/ServerBar'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import StateTreeDebugView from './components/StateTreeDebugView'
import ContactsModal from './components/ContactsModal'
import Spacer from './components/Spacer'
import useNavigation from './hooks/useNavigation'
import useCurrentServerAndRoom from './hooks/useCurrentServerAndRoom'
import useModal from './hooks/useModal'
import useRoomName from './hooks/useRoomName'
import useLoginStatus from './hooks/useLoginStatus'

const serverBarVerticalPadding = '12px'

const Layout = styled.div`
    display: flex;
    height: 100vh;
`

const HeaderAndContent = styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;
`

const Content = styled.div`
    height: calc(100vh - 50px);
    max-height: calc(100vh - 50px);
    display: flex;
    flex-grow: 1;
    font-family: 'Noto Sans KR';
    font-weight: 300;
    color: ${props => props.theme.color.white};
`

const SSidebar = styled(Sidebar)`
    height: 100%;
`

const SServerBar = styled(ServerBar)`
    width: 72px;
    min-width: 72px;
    height: calc(100% - 2 * ${props => serverBarVerticalPadding});
    background: ${props => props.theme.color.grey[600]};
`

const SChat = styled(Chat)`
    flex-grow: 1;
    padding-left: 16px;
`

const SStateTreeDebugView = styled(StateTreeDebugView)`
    width: 600px;
`

const MainContentArea = styled.div`
    width: 100%;
    display: flex;
    flex-direction: column;
`

const SHeaderBar = styled(HeaderBar)`
    background-color: ${props => props.theme.color.grey[200]};
    border-bottom: 2px solid ${props => props.theme.color.grey[300]};
    height: 48px;
    width: 100%;
`

function Main() {
    const { onDismiss: onDismissContactsModal } = useModal('contacts')
    let { isLoggedIn } = useLoginStatus()

    if (!isLoggedIn) {
        return <Redirect to={'/signin'} />
    }

    let [showDebugView, setShowDebugView] = useState(false)
    let onClickShowDebugView = useCallback(() => {
        setShowDebugView(!showDebugView)
    }, [showDebugView, setShowDebugView])

    return (
        <Layout>
            <SServerBar verticalPadding={serverBarVerticalPadding} />
            <HeaderAndContent>
                <SHeaderBar onClickShowDebugView={onClickShowDebugView} />
                <Content>
                    <Sidebar />
                    <SChat />
                    {showDebugView && <SStateTreeDebugView />}
                </Content>
            </HeaderAndContent>

            <ContactsModal onDismiss={onDismissContactsModal} />
        </Layout>
    )
}

const HeaderBarContainer = styled.div`
    display: flex;
`

const ServerTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${props => props.theme.color.white};
    background-color: ${props => props.theme.color.grey[400]};
    width: calc(${props => props.theme.chatSidebarWidth} - 18px);
    height: calc(100% - 12px);
`

const ChatTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${props => props.theme.color.white};
    width: calc(${props => props.theme.chatSidebarWidth} - 18px);
    height: calc(100% - 12px);
`

const SCodeIcon = styled(CodeIcon)`
    padding: 12px;
    cursor: pointer;
`

function HeaderBar({ onClickShowDebugView, className }) {
    const { selectedServer, selectedRoom } = useNavigation()
    const { currentRoom, currentServer } = useCurrentServerAndRoom()
    const roomName = useRoomName(selectedServer, selectedRoom)
    return (
        <HeaderBarContainer className={className}>
            <ServerTitle>{currentServer && currentServer.name} /</ServerTitle>
            <ChatTitle>{currentRoom && roomName}</ChatTitle>
            <Spacer size="flex" />
            <SCodeIcon style={{ color: 'white' }} onClick={onClickShowDebugView} />
        </HeaderBarContainer>
    )
}

export default Main
