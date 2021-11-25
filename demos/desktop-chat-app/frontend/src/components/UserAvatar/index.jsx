import React, { useState, useEffect } from 'react'
import styled from 'styled-components'
import * as tinycolor from 'tinycolor2'
import { useRedwood } from '@redwood.dev/client/react'
import strToColor from '../../utils/strToColor'
import Image from '../Image'
import useUsers from '../../hooks/useUsers'
import useAddressBook from '../../hooks/useAddressBook'
import useNavigation from '../../hooks/useNavigation'

const Avatar = styled(Image)`
    user-select: none;
    border-radius: 9999px;
    width: 40px;
    height: 40px;
`

const TextAvatar = styled.div`
    user-select: none;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: ${(props) =>
        tinycolor(strToColor(props.text)).darken(10).desaturate(25)} !important;
    font-weight: 700;
    border-radius: 9999px;
    width: 40px;
    height: 40px;
    font-size: 1.1rem;
    line-height: 1rem;
    user-select: none;
`

function UserAvatar({ address, className, ...props }) {
    const [username, setUsername] = useState(null)
    const [photoURL, setPhotoURL] = useState(null)
    const { selectedStateURI } = useNavigation()
    const { users, usersStateURI } = useUsers(selectedStateURI)
    const addressBook = useAddressBook()
    const { httpHost } = useRedwood()

    useEffect(() => {
        if (users && users[address]) {
            setUsername(users[address].username)
            if (users[address].photo) {
                setPhotoURL(
                    `${httpHost}/users/${address}/photo?state_uri=${usersStateURI}&${Date.now()}`,
                )
            } else {
                setPhotoURL(null)
            }
        } else {
            setUsername(null)
            setPhotoURL(null)
        }
    }, [users, httpHost, address, usersStateURI])

    if (photoURL) {
        return <Avatar className={className} src={photoURL} {...props} />
    }
    const text = addressBook[address] || username || address || ''
    return (
        <TextAvatar className={className} text={text} {...props}>
            <div>{(text || '').slice(0, 1).toUpperCase()}</div>
        </TextAvatar>
    )
}

export default UserAvatar
