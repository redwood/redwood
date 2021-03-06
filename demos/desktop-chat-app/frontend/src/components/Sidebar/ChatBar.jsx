import React, { useState, useCallback, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import { Avatar } from '@material-ui/core'
import { AddCircleOutline as AddIcon } from '@material-ui/icons'
import moment from 'moment'
import * as RedwoodReact from 'redwood.js/dist/module/react'

import GroupItem from './GroupItem'
import Modal, { ModalTitle, ModalContent, ModalActions } from '../Modal'
import Button from '../Button'
import Input from '../Input'
// import useRedwood from '../../hooks/useRedwood'
// import useStateTree from '../../hooks/useStateTree'
import useModal from '../../hooks/useModal'
import useAPI from '../../hooks/useAPI'
import useNavigation from '../../hooks/useNavigation'

import addChat from './assets/add_chat.svg'
import avatarPlaceholder from './assets/speech-bubble.svg'

const { useRedwood, useStateTree } = RedwoodReact

const ChatBarWrapper = styled.div`
    display: flex;
    flex-direction: column;
    background: ${props => props.theme.color.grey[400]};
`

const SGroupItem = styled(GroupItem)`
    cursor: pointer;
    transition: .15s ease-in-out all;

    border-radius: 8px;
    margin: 6px;

    &:hover {
        background: ${props => props.theme.color.grey[300]};
    }
`

const SControlWrapper = styled.div`
    display: flex;
    align-items: center;
    justify-content: start;
    cursor: pointer;
    transition: .15s ease-in-out all;
    background: transparent;
    height: 40px;
    padding-left: 12px;
    color: ${props => props.theme.color.indigo[500]};
    font-weight: 500;

    &:hover {
        background: ${props => props.theme.color.grey[300]};
    }
`

const Spacer = styled.div`
    flex-grow: 1;
`

const SAddIcon = styled(AddIcon)`
    margin-right: 10px;
`

function ChatBar({ className }) {
    const { selectedServer, selectedStateURI, navigate } = useNavigation()
    const { stateTrees } = useRedwood()
    const serverState = useStateTree(!!selectedServer ? `${selectedServer}/registry` : null)
    const { onPresent: onPresentNewChatModal, onDismiss: onDismissNewChatModal } = useModal('new chat')
    const theme = useTheme()

    const onClickCreateNewChat = useCallback(() => {
        if (!selectedServer) {
            return
        }
        onPresentNewChatModal()
    }, [selectedServer, onPresentNewChatModal])

    let serverRooms = ((!!serverState ? serverState.rooms : []) || []).filter(room => !!room)

    return (
        <ChatBarWrapper className={className}>
            {serverRooms.map(roomStateURI => {
                let messages = (stateTrees[roomStateURI] || {}).messages || []
                let mostRecentMessageText
                if (messages.length > 0) {
                    mostRecentMessageText = messages[messages.length - 1].text
                }
                return <ChatBarItem
                            key={roomStateURI}
                            selectedServer={selectedServer}
                            stateURI={roomStateURI}
                            selectedStateURI={selectedStateURI}
                            navigate={navigate}
                            mostRecentMessageText={mostRecentMessageText}
                />
            })}

            <Spacer />

            {!!selectedServer &&
                <SControlWrapper onClick={onClickCreateNewChat}>
                    <SAddIcon style={{ color: theme.color.indigo[500] }} /> New chat
                </SControlWrapper>
            }
            <NewChatModal selectedServer={selectedServer} serverRooms={serverRooms} onDismiss={onDismissNewChatModal} navigate={navigate} />
        </ChatBarWrapper>
    )
}

function ChatBarItem({ stateURI, selectedServer, selectedStateURI, navigate, mostRecentMessageText }) {
    const chatState = useStateTree(stateURI)
    let [ _, roomName ] = stateURI.split('/')
    const [latestMessageTime, setLatestMessageTime] = useState(null)

    let latestTimestamp
    if (chatState && chatState.messages) {
      if (chatState.messages[chatState.messages.length - 1]) {
        latestTimestamp = chatState.messages[chatState.messages.length - 1].timestamp
      }
    }

    useEffect(() => {
      const intervalId = setInterval(() => {
        if (latestTimestamp) {
          let timeAgo = moment.unix(latestTimestamp).fromNow()
          if (timeAgo === 'a few seconds ago') { timeAgo = 'moments ago' }
          setLatestMessageTime(timeAgo)
        }
      }, 1000)
      return () => {
        clearInterval(intervalId)
      }
    }, [chatState])

    return (
        <SGroupItem
            key={stateURI}
            selected={selectedStateURI === stateURI}
            onClick={() => navigate(selectedServer, roomName)}
            name={roomName}
            text={mostRecentMessageText}
            time={latestMessageTime}
            avatar={avatarPlaceholder}
        />
    )
}

const SInput = styled(Input)`
    width: 460px;
`

function NewChatModal({ selectedServer, serverRooms, onDismiss, navigate }) {
    const [newChatName, setNewChatName] = useState('')
    const api = useAPI()

    const onClickCreate = useCallback(async () => {
        if (!api) { return }
        try {
            await api.createNewChat(selectedServer, newChatName, serverRooms)
            onDismiss()
            navigate(selectedServer, newChatName)
        } catch (err) {
            console.error(err)
        }
    }, [api, selectedServer, newChatName, serverRooms, navigate])

    function onChangeNewChatName(e) {
        setNewChatName(e.target.value)
    }

    function onKeyDown(e) {
        if (e.code === 'Enter') {
            e.stopPropagation()
            onClickCreate()
            setNewChatName('')
        }
    }

    function closeModal() {
      setNewChatName()
      onDismiss()
    }

    return (
        <Modal modalKey="new chat">
            <ModalTitle closeModal={closeModal}>Create a Chat</ModalTitle>
            <ModalContent>
                <Input
                  value={newChatName}
                  onChange={onChangeNewChatName}
                  onKeyDown={onKeyDown}
                  label={'Chat Name'}
                  width={'460px'}
                />
            </ModalContent>
            <ModalActions>
                <Button primary onClick={onClickCreate}>Create</Button>
            </ModalActions>
        </Modal>
    )
}

export default ChatBar