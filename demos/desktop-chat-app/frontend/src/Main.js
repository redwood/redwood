import React, { useState, useEffect, useCallback, memo } from 'react'
import styled from 'styled-components'
import { Redirect } from 'react-router-dom'
import { Code as CodeIcon } from '@material-ui/icons'
import { ToastContainer } from 'react-toastify'
import { useRedwood } from './components/redwood.js/dist/main/react'
import 'react-toastify/dist/ReactToastify.css'

import ServerBar from './components/Sidebar/ServerBar'
import Loading from './components/Account/Loading'
import Sidebar from './components/Sidebar'
import HeaderBar from './components/Main/HeaderBar'
import Chat from './components/Chat'
import NotificationMounter from './components/Notifications/Mounter'
// import Chat from './components/NewChat'
import StateTreeDebugView from './components/StateTreeDebugView'
import ContactsModal from './components/ContactsModal'
import useNavigation from './hooks/useNavigation'
import useCurrentServerAndRoom from './hooks/useCurrentServerAndRoom'
import useModal from './hooks/useModal'
import useRoomName from './hooks/useRoomName'
import useLoginStatus from './hooks/useLoginStatus'

const serverBarVerticalPadding = '12px'

const Layout = memo(styled.div`
    display: flex;
    height: 100vh;
    overflow: hidden;
`)

const HeaderAndContent = memo(styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;
`)

const Content = memo(styled.div`
    height: calc(100vh - 50px);
    max-height: calc(100vh - 50px);
    display: flex;
    flex-grow: 1;
    font-family: 'Noto Sans KR';
    font-weight: 300;
    color: ${(props) => props.theme.color.white};
`)

const SServerBar = memo(styled(ServerBar)`
    width: 72px;
    min-width: 72px;
    height: calc(100% - 2 * ${() => serverBarVerticalPadding});
    background: ${(props) => props.theme.color.grey[600]};
`)

const SChat = memo(styled(Chat)`
    flex-grow: 1;
    padding-left: 16px;
`)

const SStateTreeDebugView = memo(styled(StateTreeDebugView)`
    width: 600px;
`)

const SHeaderBar = memo(styled(HeaderBar)`
    background-color: ${(props) => props.theme.color.grey[200]};
    border-bottom: 2px solid ${(props) => props.theme.color.grey[300]};
    height: 48px;
    width: 100%;
`)

// APIProvider, PeersProvider, ServerAndRoomInfoProvider, NavigationProvider

function Main(props) {
    const { onDismiss: onDismissContactsModal } = useModal('contacts')
    const [isLoading, setIsLoading] = useState(true)
    const [shouldRedirect, setShouldRedirect] = useState(false)
    const [showDebugView, setShowDebugView] = useState(false)
    const { isLoggedIn, profilesFetched } = useLoginStatus()
    const { nodeIdentities } = useRedwood()

    const { renderCountRef } = props
    if (renderCountRef.current) {
        renderCountRef.current += 1
        console.log(renderCountRef.current)
    }

    const onClickShowDebugView = useCallback(() => {
        setShowDebugView(!showDebugView)
    }, [showDebugView, setShowDebugView])

    useEffect(() => {
        if (nodeIdentities) {
            setIsLoading(false)
        }
    }, [nodeIdentities])

    useEffect(() => {
        if (profilesFetched) {
            if (!isLoggedIn) {
                setShouldRedirect(true)
            }
        }
    }, [profilesFetched, isLoggedIn])

    if (shouldRedirect) {
        if ((props.profileNames || []).length === 0) {
            return <Redirect to="/signup" />
        }
        return <Redirect to="/profiles" />
    }

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
                <NotificationMounter />
            </HeaderAndContent>

            <ContactsModal onDismiss={onDismissContactsModal} />
            <ToastContainer pauseOnFocusLoss={false} newestOnTop />
            {isLoading ? <Loading text="Loading account and chats..." /> : null}
        </Layout>
    )
}

const HeaderBarContainer = memo(styled.div`
    display: flex;
`)

const ServerTitle = memo(styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${(props) => props.theme.color.white};
    background-color: ${(props) => props.theme.color.grey[400]};
    width: calc(${(props) => props.theme.chatSidebarWidth} - 18px);
    height: calc(100% - 12px);
`)

const ChatTitle = memo(styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${(props) => props.theme.color.white};
    white-space: nowrap;
    text-overflow: none;
    height: calc(100% - 12px);
`)

const SCodeIcon = memo(styled(CodeIcon)`
    padding: 12px;
    cursor: pointer;
`)

// function HeaderBar({ onClickShowDebugView, className }) {
//     const { selectedServer, selectedRoom } = useNavigation()
//     const { currentRoom, currentServer } = useCurrentServerAndRoom()
//     const roomName = useRoomName(selectedServer, selectedRoom)

//     console.log('render - header bar')

//     return (
//         <HeaderBarContainer className={className}>
//             <ServerTitle>{currentServer && currentServer.name} /</ServerTitle>
//             <ChatTitle>{currentRoom && roomName}</ChatTitle>
//             <SCodeIcon
//                 style={{ color: 'white', marginLeft: 'auto' }}
//                 onClick={onClickShowDebugView}
//             />
//         </HeaderBarContainer>
//     )
// }

export default Main
