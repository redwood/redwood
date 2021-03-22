import React, { useState, useCallback, useRef } from 'react'
import styled from 'styled-components'

import UserAvatar from '../UserAvatar'

const Avatar = styled.img`
    border-radius: 9999px;
`

function CurrentUserAvatar({ className, nodeAddress, username, photoURL }) {
    return <UserAvatar address={nodeAddress} username={username} photoURL={photoURL} className={className} />
}

export default CurrentUserAvatar
