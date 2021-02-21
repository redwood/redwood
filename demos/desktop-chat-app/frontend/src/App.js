import React, { useState, useRef } from 'react'
import styled from 'styled-components'

import ServerBar from './components/Sidebar/ServerBar'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import StateTreeDebugView from './components/StateTreeDebugView'
import useNavigation from './hooks/useNavigation'

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
    max-height: calc(100vh - 50px);
    display: flex;
    flex-grow: 1;
    font-family: 'Noto Sans KR';
    font-weight: 300;
    color: ${props => props.theme.color.white};
`

const SServerBar = styled(ServerBar)`
    width: 72px;
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

function Login(props) {
  const mnemonicInput = useRef(null)

  const login = async (event) => {
    event.preventDefault()
    let resp = await (await fetch('http://localhost:54231/api/login', {
      method: 'POST',
      headers: {
          'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        mnemonic: mnemonicInput.current.value,
      }),
    })).json()


    props.setIsLoggedIn(true)

    console.log(resp)
  }

  return (
    <form onSubmit={login}>
      <label>Mnemonic <input ref={mnemonicInput} placeholder="Mnemonic..." /></label>
      <button type="submit">Login</button>
    </form>
  )
}

function App() {
  const [isLoggedin, setIsLoggedIn] = useState(null)

  // if (true) {
  //   return <Login
  //     isLoggedin={isLoggedin}
  //     setIsLoggedIn={setIsLoggedIn}
  //   />
  // }

    return (
        <Layout>
            <SServerBar verticalPadding={serverBarVerticalPadding} />
            <HeaderAndContent>
                <SHeaderBar />
                <Content>
                    <Sidebar />
                    <SChat />
                    <SStateTreeDebugView />
                </Content>
            </HeaderAndContent>
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

function HeaderBar({ className }) {
    const { selectedServer, selectedRoom } = useNavigation()
    return (
        <HeaderBarContainer className={className}>
            <ServerTitle>{selectedServer}/</ServerTitle>
            {selectedRoom &&
                <ChatTitle>{selectedRoom}</ChatTitle>
            }
        </HeaderBarContainer>
    )
}

export default App
