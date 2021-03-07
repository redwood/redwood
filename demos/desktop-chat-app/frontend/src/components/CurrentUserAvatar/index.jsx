import React, { useState, useCallback, useRef } from 'react'
import styled from 'styled-components'
import UserAvatar from '../UserAvatar'
import { useRedwood, useStateTree } from 'redwood/dist/main/react'
import useNavigation from '../../hooks/useNavigation'

const Avatar = styled.img`
    border-radius: 9999px;
`

function CurrentUserAvatar({ className }) {
    let { nodeIdentities } = useRedwood()
    let { selectedServer } = useNavigation()
    let registry = useStateTree(!!selectedServer ? `${selectedServer}/registry` : null)

    let nodeAddress = !!nodeIdentities && nodeIdentities.length > 0 ? nodeIdentities[0].address.toLowerCase() : null

    let username, userPhotoURL
    if (registry && registry.users && registry.users[nodeAddress]) {
        username = registry.users[nodeAddress].username
        if (registry.users[nodeAddress].photo) {
            userPhotoURL = `http://localhost:8080/users/${nodeAddress}/photo?state_uri=${selectedServer}/registry&${Date.now()}`
        }
    }
    return <UserAvatar address={nodeAddress} username={username} photoURL={userPhotoURL} className={className} />
}

export default CurrentUserAvatar
