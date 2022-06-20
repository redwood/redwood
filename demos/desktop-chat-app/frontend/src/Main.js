import React, { useState, useRef, useEffect, useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { Redirect, useHistory } from 'react-router-dom'
import { Code as CodeIcon, InsertDriveFile as FileIcon } from '@material-ui/icons'
import { ToastContainer, toast } from 'react-toastify'
import 'react-toastify/dist/ReactToastify.css'

import { useRedwood, useStateTree } from '@redwood.dev/react'

import ServerBar from './components/Sidebar/ServerBar'
import Loading from './components/Account/Loading'
import UserAvatar from './components/UserAvatar'
import NormalizeMessage from './components/ChatHelpers/NormalizeMessage'
import ToastCloseBtn from './components/Toast/ToastCloseBtn'
import ToastContent from './components/Toast/ToastContent'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import StateTreeDebugView from './components/StateTreeDebugView'
import FileView from './components/FileView'
import ContactsModal from './components/ContactsModal'
import Spacer from './components/Spacer'
import useNavigation from './hooks/useNavigation'
import useCurrentServerAndRoom from './hooks/useCurrentServerAndRoom'
import useModal from './hooks/useModal'
import useRoomName from './hooks/useRoomName'
import useUsers from './hooks/useUsers'
import useLoginStatus from './hooks/useLoginStatus'
import useServerAndRoomInfo from './hooks/useServerAndRoomInfo'
import useAddressBook from './hooks/useAddressBook'
import notificationSound from './assets/notification-sound.mp3'
import notificationGilfoyle from './assets/notification-gilfoyle.mp3'

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

const SFileView = styled(FileView)`
    width: 564px;
`

const SStateTreeDebugView = styled(StateTreeDebugView)`
    width: 564px;
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

function Main(props) {
	const { onDismiss: onDismissContactsModal } = useModal('contacts')
	const { selectedStateURI, navigate } = useNavigation()
	const { servers, rooms } = useServerAndRoomInfo()
	const [isLoading, setIsLoading] = useState(true)
	const [shouldRedirect, setShouldRedirect] = useState(false)
	const [changeNotificationSound, setChangeNotificationSound] = useState(false)
	let { nodeIdentities } = useRedwood()

	const roomKeys = Object.keys(rooms || {}).filter((key) => key !== 'chat.local/address-book')

	useEffect(() => {
		function switchNotificationSound(event) {
			if (event.ctrlKey && event.key === '2') {
				setChangeNotificationSound(!changeNotificationSound)
				if (!changeNotificationSound) {
					const audio = new Audio(notificationGilfoyle)
					audio.play()
				} else {
					const audio = new Audio(notificationSound)
					audio.play()
				}
			}

		}

		document.addEventListener('keypress', switchNotificationSound)

		return () => {
			document.removeEventListener('keypress', switchNotificationSound)
		}
	})

	useEffect(() => {
		if (nodeIdentities) {
			setIsLoading(false)
		}
	}, [nodeIdentities])

	let { isLoggedIn, profilesFetched } = useLoginStatus()

	useEffect(() => {
		if (profilesFetched) {
			if (!isLoggedIn) {
				setShouldRedirect(true)
			}
		}
	}, [profilesFetched])


    let [showFileView, setShowFileView] = useState(false)
    let [showDebugView, setShowDebugView] = useState(false)

    let onClickShowFileView = useCallback(() => {
        if (showFileView) {
            setShowFileView(false)
        } else {
            setShowFileView(true)
            setShowDebugView(false)
        }
    }, [showFileView, setShowFileView])

    let onClickShowDebugView = useCallback(() => {
        if (showDebugView) {
            setShowDebugView(false)
        } else {
            setShowDebugView(true)
            setShowFileView(false)
        }
    }, [showDebugView, setShowDebugView])

	if (shouldRedirect) {
		if ((props.profileNames || []).length === 0) {
			return <Redirect to={'/signup'} />
		}
		return <Redirect to={'/profiles'} />
	}

    return (
        <Layout>
            <SServerBar verticalPadding={serverBarVerticalPadding} />
            <HeaderAndContent>
                <SHeaderBar
                    showFileView={showFileView}
                    onClickShowFileView={onClickShowFileView}
                    showDebugView={showDebugView}
                    onClickShowDebugView={onClickShowDebugView}
                />
                <Content>
                    <Sidebar />
                    <SChat />
                    {showFileView && <SFileView />}
                    {showDebugView && <SStateTreeDebugView />}
                </Content>
				{ roomKeys.map((key) => <NotificationMount
											changeNotificationSound={changeNotificationSound}
											navigate={navigate}
											selectedStateURI={selectedStateURI}
											roomPath={key}
											key={key}
										/>) }
            </HeaderAndContent>

            <ContactsModal onDismiss={onDismissContactsModal} />
			<ToastContainer />
			{ isLoading ? <Loading text={'Loading account and chats...'} /> : null }
        </Layout>
    )
}

const SToastContent = styled(ToastContent)`

`
const ToastLeft = styled.div`
	display: flex;
	align-items: center;
	justify-content: center;
`

const ToastRight = styled.div`
	display: flex;
	flex-direction: column;
	padding-left: 12px;
	width: 217px;
	padding-bottom: 4px;
`

const SToastRoom = styled.div`
	font-size: 10px;
	color: rgba(255,255,255, .6);
	span {
		font-size: 12px;
		color: ${props => props.themePrimaryColor};
		text-decoration: underline;
	}
`
const SToastUser = styled.div`
	padding-top: 4px;
	font-size: 10px;
`

function fireNotificationAlert({
	roomPath,
	messageText,
	displayName,
	themePrimaryColor,
	navigateToMessage,
	setToastId,
	changeNotificationSound,
}) {
	let parsedDisplayName = displayName

	if (displayName.length > 25) {
		parsedDisplayName = `${displayName.substring(0, 25)}...`
	}

	let toastId = toast(<SToastContent onClick={navigateToMessage}>
		<ToastLeft>
			<UserAvatar address={displayName} />
		</ToastLeft>
		<ToastRight>
			<SToastRoom themePrimaryColor={themePrimaryColor}>New message in <span>{roomPath}</span>!</SToastRoom>
			<NormalizeMessage isNotification style={{ fontSize: 14 }} msgText={messageText} />
			<SToastUser>Sent by {parsedDisplayName}</SToastUser>
		</ToastRight>
	</SToastContent>, {
		autoClose: 4500,
		style: {
			background: '#2a2d32',
		},
		closeButton: ToastCloseBtn,
	})

	setToastId(toastId)

	if (changeNotificationSound) {
		const audio = new Audio(notificationGilfoyle)
		audio.play()
	} else {
		const audio = new Audio(notificationSound)
		audio.play()
	}
}

// Used to mount the room state and notify users when new messages come in
function NotificationMount(props) {
	const [roomState] = useStateTree(props.roomPath)
	let { users } = useUsers(props.roomPath)
	const messages = (roomState || {}).messages || []
	const numMessages = messages.length
	const latestMessage = messages[messages.length - 1] || {}
	const addressBook = useAddressBook()
	const theme = useTheme()
	const [toastId, setToastId] = useState(null)

	const [isLoading, setIsLoading ] = useState(true)

	const [server, room] = props.roomPath.split('/')

	const navigateToMessage = () => {
		props.navigate(server, room)
		toast.dismiss(toastId)
	}
	
	useEffect(() => {
		if (props.roomPath === props.selectedStateURI && document.hasFocus()) {
			return
		}

		if (isLoading) {
			setIsLoading(false)
			return
		}

		if (messages.length === 0) {
			return
		}

		let userAddress = (latestMessage.sender || "").toLowerCase()
		let user = (users && users[userAddress]) || {}
		let displayName = addressBook[userAddress] || user.username || latestMessage.sender
		
		fireNotificationAlert({
			roomPath: props.roomPath,
			messageText: latestMessage.text,
			displayName,
			themePrimaryColor: theme.color.indigo[500],
			navigateToMessage,
			setToastId,
			changeNotificationSound: props.changeNotificationSound,
		})
	}, [numMessages])

	return <div style={{ display: 'none' }}></div>
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
	// width: calc(${props => props.theme.chatSidebarWidth} - 18px);
	white-space: nowrap;
	text-overflow: none;
	height: calc(100% - 12px);
    flex-grow: 1;
    border-right: ${props => props.showDivider ? '2px solid ' + props.theme.color.grey[300] : 'none'};
`

const SCodeIcon = styled(CodeIcon)`
    padding: 12px;
    cursor: pointer;
`

const SFileIcon = styled(FileIcon)`
    padding: 12px;
    cursor: pointer;
`

const RightSidebarHeader = styled.div`
    width: 564px;
    justify-self: flex-end;
    display: flex;
`

const RightSidebarTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${props => props.theme.color.white};
    flex-grow: 1;
`

const RightSidebarButtons = styled.div`
`

function HeaderBar({ showFileView, onClickShowFileView, showDebugView, onClickShowDebugView, className }) {
    const theme = useTheme()
    const { selectedServer, selectedRoom } = useNavigation()
    const { currentRoom, currentServer } = useCurrentServerAndRoom()
    const roomName = useRoomName(selectedServer, selectedRoom)

    let rightSidebarTitle
    if (showFileView) {
        rightSidebarTitle = 'Files'
    } else if (showDebugView) {
        rightSidebarTitle = 'State Debugger'
    }

    return (
        <HeaderBarContainer className={className}>
            <ServerTitle>{currentServer && currentServer.name} /</ServerTitle>
            <ChatTitle showDivider={!!rightSidebarTitle}>{currentRoom && roomName}</ChatTitle>
            {/* <Spacer size="flex" /> */}
            <RightSidebarHeader>
                <RightSidebarTitle>{rightSidebarTitle}</RightSidebarTitle>
                <RightSidebarButtons>
                    <SFileIcon style={{ color: showFileView  ? theme.color.green[500] : 'white', marginLeft: 'auto' }} onClick={onClickShowFileView} />
                    <SCodeIcon style={{ color: showDebugView ? theme.color.green[500] : 'white' }} onClick={onClickShowDebugView} />
                </RightSidebarButtons>
            </RightSidebarHeader>
        </HeaderBarContainer>
    )
}

export default Main
