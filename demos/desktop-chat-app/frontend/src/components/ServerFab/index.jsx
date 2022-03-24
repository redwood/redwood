import React, { useCallback } from 'react'
import styled, { useTheme } from 'styled-components'
import { Fab } from '@material-ui/core'
import { Face as FaceIcon } from '@material-ui/icons'
import { useRedwood, useStateTree } from '@redwood.dev/react'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import useServerRegistry from '../../hooks/useServerRegistry'
import useNavigation from '../../hooks/useNavigation'
import strToColor from '../../utils/strToColor'

const SFab = styled(Fab)`
    width: 50px !important;
    height: 50px !important;
    transition: .12s ease-in-out all !important;
    background-color: ${props => props.$color || strToColor(props.text)} !important;
    color: ${props => props.theme.color.white} !important;
    font-weight: 700 !important;
    font-size: 1.1rem !important;
    overflow: hidden;

    img {
      height: 50px;
      border-radius: 100%;
    }

    &:hover {
        // border-radius: 20px !important;
        transform: scale(1.1);
    }
`

function ServerFab({ serverName, navigateOnClick, className }) {
    const { servers } = useServerAndRoomInfo()
    const { registry, registryStateURI } = useServerRegistry(serverName)
    const { navigate } = useNavigation()
    const theme = useTheme()
    const { httpHost } = useRedwood()

    const onClick = useCallback(() => {
        if (navigateOnClick) {
            navigate(serverName, null)
        }
    }, [navigate, navigateOnClick, serverName])

    if ((servers[serverName] || {}).isDirectMessage) {
        return (
            <SFab $color={theme.color.green[500]} onClick={onClick}>
                <SFaceIcon />
            </SFab>
        )

    } else if (registry && registry.iconImg) {
        return (
            <SFab text={serverName} onClick={onClick} className={className}>
                <img src={`${httpHost}/iconImg?state_uri=${registryStateURI}`} />
            </SFab>
        )
    }
    return <SFab onClick={onClick} text={serverName} className={className}>{serverName.slice(0, 1)}</SFab>
}

const SFaceIcon = styled(FaceIcon)`
    && {
        width: 40px;
        height: 40px;
    }
`

function DMButton() {
    const theme = useTheme()
    return (
        <SFab $color={theme.color.green[500]}>
            <SFaceIcon />
        </SFab>
    )
}

export {
    ServerFab,
    DMButton,
}

