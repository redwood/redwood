import { useState, useEffect, useCallback } from 'react'
import styled from 'styled-components'
import { Redirect } from 'react-router-dom'
import { ToastContainer } from 'react-toastify'
import 'react-toastify/dist/ReactToastify.css'

import ServerBar from './components/Sidebar/ServerBar'
import Loading from './components/Account/Loading'
import Sidebar from './components/Sidebar'
import HeaderBar from './components/Main/HeaderBar'
import Chat from './components/Chat'
import LogicMounter from './components/LogicMounter'
import StateTreeDebugView from './components/StateTreeDebugView'
import ContactsModal from './components/ContactsModal'
import useModal from './hooks/useModal'

const serverBarVerticalPadding = '12px'

const Layout = styled.div`
    display: flex;
    height: 100vh;
    overflow: hidden;
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
    color: ${(props) => props.theme.color.white};
`

const SServerBar = styled(ServerBar)`
    width: 72px;
    min-width: 72px;
    height: calc(100% - 2 * ${() => serverBarVerticalPadding});
    background: ${(props) => props.theme.color.grey[600]};
`

const SChat = styled(Chat)`
    flex-grow: 1;
    padding-left: 16px;
`

const SStateTreeDebugView = styled(StateTreeDebugView)`
    width: 600px;
`

const SHeaderBar = styled(HeaderBar)`
    background-color: ${(props) => props.theme.color.grey[200]};
    border-bottom: 2px solid ${(props) => props.theme.color.grey[300]};
    height: 48px;
    width: 100%;
`

function Main({
    isLoggedIn,
    profilesFetched,
    profileNames,
    connectionError,
    checkingLogin,
}) {
    const { onDismiss: onDismissContactsModal } = useModal('contacts')
    const [isLoading, setIsLoading] = useState(true)
    const [shouldRedirect, setShouldRedirect] = useState(false)
    const [showDebugView, setShowDebugView] = useState(false)

    const onClickShowDebugView = useCallback(() => {
        setShowDebugView(!showDebugView)
    }, [showDebugView, setShowDebugView])

    useEffect(() => {
        if (profilesFetched) {
            if (!isLoggedIn) {
                setShouldRedirect(true)
            }
        }
    }, [profilesFetched, isLoggedIn])

    if (checkingLogin) {
        return <Redirect to="/loading" />
    }
    if (connectionError) {
        return <Redirect to="/connection-error" />
    }

    if (shouldRedirect) {
        if ((profileNames || []).length === 0) {
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
                <LogicMounter
                    isLoggedIn={isLoggedIn}
                    setIsLoading={setIsLoading}
                />
            </HeaderAndContent>

            <ContactsModal onDismiss={onDismissContactsModal} />
            <ToastContainer pauseOnFocusLoss={false} newestOnTop />
            {isLoading ? <Loading text="Loading account and chats..." /> : null}
        </Layout>
    )
}

export default Main
