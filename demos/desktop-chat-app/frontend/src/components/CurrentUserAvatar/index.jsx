import React, { useState, useCallback, useRef } from 'react'
import styled from 'styled-components'
import UserAvatar from '../UserAvatar'
import useRedwood from '../../hooks/useRedwood'
import useNavigation from '../../hooks/useNavigation'
import useStateTree from '../../hooks/useStateTree'

const Avatar = styled.img`
    border-radius: 9999px;
`

function CurrentUserAvatar({ className }) {
    let { nodeAddress } = useRedwood()
    let { selectedServer } = useNavigation()
    let registry = useStateTree(!!selectedServer ? `${selectedServer}/registry` : null)

    nodeAddress = !!nodeAddress ? nodeAddress.toLowerCase() : null

    let username, userPhotoURL
    if (registry && registry.users && registry.users[nodeAddress]) {
        username = registry.users[nodeAddress].username
        if (registry.users[nodeAddress].photo) {
            userPhotoURL = `http://localhost:8080/users/${nodeAddress}/photo?state_uri=${selectedServer}/registry`
        }
    }
    return <UserAvatar address={nodeAddress} username={username} photoURL={userPhotoURL} className={className} />
}

export default CurrentUserAvatar
