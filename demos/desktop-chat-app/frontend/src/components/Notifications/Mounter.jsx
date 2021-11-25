import React, { useState, useCallback, useEffect, useMemo, useRef } from 'react'
import styled, { useTheme } from 'styled-components'

import { toast } from 'react-toastify'
import { useStateTree } from '../redwood.js/dist/main/react'

import ToastCloseBtn from '../Toast/ToastCloseBtn'
import ToastContent from '../Toast/ToastContent'
import UserAvatar from '../UserAvatar'
import NormalizeMessage from '../Chat/NormalizeMessage'

import notificationSound from '../../assets/notification-sound.mp3'
import notificationGilfoyle from '../../assets/notification-gilfoyle.mp3'
import useUsers from '../../hooks/useUsers'
import useAddressBook from '../../hooks/useAddressBook'
import useNavigation from '../../hooks/useNavigation'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'

const SToastRoom = styled.div`
    font-size: 10px;
    color: rgba(255, 255, 255, 0.6);
    span {
        font-size: 12px;
        color: ${(props) => props.themePrimaryColor};
        text-decoration: underline;
    }
`

const ToastLeft = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
`

const SToastUser = styled.div`
    padding-top: 4px;
    font-size: 10px;
`

const ToastRight = styled.div`
    display: flex;
    flex-direction: column;
    padding-left: 12px;
    width: 217px;
    padding-bottom: 4px;
`

function Mounter() {
    const [changeNotificationSound, setChangeNotificationSound] =
        useState(false)
    const { selectedStateURI, navigate } = useNavigation()
    const { rooms } = useServerAndRoomInfo()
    const theme = useTheme()

    const roomKeys = useMemo(
        () =>
            Object.keys(rooms || {}).filter(
                (key) => key !== 'chat.local/address-book',
            ),
        [rooms],
    )

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

    return (
        <>
            {roomKeys.map((room) => (
                <NotificationMount
                    changeNotificationSound={changeNotificationSound}
                    navigate={navigate}
                    selectedStateURI={selectedStateURI}
                    roomPath={room}
                    key={room}
                    theme={theme}
                />
            ))}
        </>
    )
}

// Used to mount the room state and notify users when new messages come in
function NotificationMount({
    roomPath,
    navigate,
    selectedStateURI,
    changeNotificationSound,
    theme,
}) {
    const roomState = useStateTree(roomPath)
    const { users } = useUsers(roomPath)
    const addressBook = useAddressBook()
    const [toastId, setToastId] = useState(null)
    const [isLoading, setIsLoading] = useState(true)
    const prevMessageCountRef = useRef()

    const [server, room] = roomPath.split('/')

    const messages = useMemo(
        () => (roomState || {}).messages || [],
        [roomState],
    )
    const latestMessage = useMemo(
        () => messages[messages.length - 1] || {},
        [messages],
    )

    const displayName = useMemo(() => {
        const userAddress = (latestMessage.sender || '').toLowerCase()
        const user = (users && users[userAddress]) || {}
        return addressBook[userAddress] || user.username || latestMessage.sender
    }, [addressBook, latestMessage.sender, users])

    const navigateToMessage = useCallback(() => {
        navigate(server, room)
        toast.dismiss(toastId)
    }, [server, room, navigate, toastId])

    const fireNotificationAlert = useCallback(() => {
        if (!latestMessage.text) {
            return
        }

        let parsedDisplayName = displayName

        if (displayName.length > 25) {
            parsedDisplayName = `${displayName.substring(0, 25)}...`
        }

        const newToastId = toast(
            <ToastContent onClick={navigateToMessage}>
                <ToastLeft>
                    <UserAvatar address={displayName} />
                </ToastLeft>
                <ToastRight>
                    <SToastRoom themePrimaryColor={theme.color.indigo[500]}>
                        New message in <span>{roomPath}</span>!
                    </SToastRoom>
                    <NormalizeMessage
                        isNotification
                        style={{ fontSize: 14 }}
                        msgText={latestMessage.text}
                    />
                    <SToastUser>Sent by {parsedDisplayName}</SToastUser>
                </ToastRight>
            </ToastContent>,
            {
                autoClose: 4500,
                style: {
                    background: '#2a2d32',
                },
                closeButton: ToastCloseBtn,
            },
        )

        setToastId(newToastId)

        if (changeNotificationSound) {
            try {
                const audio = new Audio(notificationGilfoyle)
                audio.play()
            } catch (err) {
                console.log(err)
            }
        } else {
            try {
                const audio = new Audio(notificationSound)
                audio.play()
            } catch (err) {
                console.log(err)
            }
        }
    }, [
        changeNotificationSound,
        navigateToMessage,
        latestMessage.text,
        roomPath,
        theme.color.indigo,
        displayName,
    ])

    useEffect(() => {
        if (roomPath === selectedStateURI && document.hasFocus()) {
            return
        }

        if (isLoading) {
            setIsLoading(false)
            return
        }

        if (messages.length === 0) {
            return
        }
        if (prevMessageCountRef) {
            if (messages.length === prevMessageCountRef.current) {
                return
            }
        }

        fireNotificationAlert()
    }, [
        isLoading,
        messages.length,
        roomPath,
        selectedStateURI,
        fireNotificationAlert,
    ])

    useEffect(() => {
        prevMessageCountRef.current = messages.length
    }, [messages.length])

    return <div style={{ display: 'none' }}></div>
}

export default Mounter
