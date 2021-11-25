import React, { memo } from 'react'
import styled from 'styled-components'
import CodeIcon from '@material-ui/icons/Code'

import useNavigation from '../../hooks/useNavigation'
import useCurrentServerAndRoom from '../../hooks/useCurrentServerAndRoom'
import useRoomName from '../../hooks/useRoomName'

const HeaderBarContainer = styled.div`
    display: flex;
`

const ServerTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${(props) => props.theme.color.white};
    background-color: ${(props) => props.theme.color.grey[400]};
    width: calc(${(props) => props.theme.chatSidebarWidth} - 18px);
    height: calc(100% - 12px);
`

const ChatTitle = styled.div`
    font-size: 1.1rem;
    font-weight: 500;
    padding-top: 12px;
    padding-left: 18px;
    color: ${(props) => props.theme.color.white};
    white-space: nowrap;
    text-overflow: none;
    height: calc(100% - 12px);
`

const SCodeIcon = styled(CodeIcon)`
    padding: 12px;
    cursor: pointer;
    color: white;
    margin-left: auto;
`

function HeaderBar({ onClickShowDebugView, className }) {
    const { selectedServer, selectedRoom } = useNavigation()
    const { currentRoom, currentServer } = useCurrentServerAndRoom()
    const roomName = useRoomName(selectedServer, selectedRoom)

    // console.log(currentServer)

    return (
        <HeaderBarContainer className={className}>
            <ServerTitle>{currentServer && currentServer.name} /</ServerTitle>
            <ChatTitle>{currentRoom && roomName}</ChatTitle>
            <SCodeIcon onClick={onClickShowDebugView} />
        </HeaderBarContainer>
    )
}

export default HeaderBar