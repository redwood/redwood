import React, { useState, useCallback, useRef } from 'react'
import styled, { useTheme } from 'styled-components'
import * as tinycolor from 'tinycolor2'
import strToColor from '../../utils/strToColor'

const Avatar = styled.img`
    border-radius: 9999px;
    width: 40px;
    height: 40px;
`

const TextAvatar = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: ${props => tinycolor(strToColor(props.text)).darken(10).desaturate(25)} !important;
    font-weight: 700;
    border-radius: 9999px;
    width: 40px;
    height: 40px;
    font-size: 1.1rem;
    line-height: 1rem;
`

function UserAvatar({ username, address, photoURL, className }) {
    console.log({ username, address, photoURL, className })
    if (photoURL) {
        return <Avatar className={className} src={photoURL} />
    }
    let text = username ? username : address || ''
    return <TextAvatar text={text}><div>{(text || '').slice(0, 1).toUpperCase()}</div></TextAvatar>
}

export default UserAvatar
