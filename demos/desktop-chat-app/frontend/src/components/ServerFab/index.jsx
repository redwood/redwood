import { useCallback, useMemo } from 'react'
import styled, { css } from 'styled-components'
import { Fab } from '@material-ui/core'
import { Face as FaceIcon } from '@material-ui/icons'
import { useRedwood } from '@redwood.dev/client/react'
import useServerAndRoomInfo from '../../hooks/useServerAndRoomInfo'
import useServerRegistry from '../../hooks/useServerRegistry'
import useNavigation from '../../hooks/useNavigation'
import strToColor from '../../utils/strToColor'

const DMBackgroundColor = css`
    background-color: ${({ theme }) => theme.color.green[500]} !important;
`

const GenerateBackgroundColor = css`
    background-color: ${({ $text, theme }) =>
        $text ? strToColor($text) : theme.color.green[500]} !important;
`

const SFab = styled(Fab)`
    width: 50px !important;
    height: 50px !important;
    transition: 0.12s ease-in-out all !important;
    ${({ $isDirectMessage }) =>
        $isDirectMessage ? DMBackgroundColor : GenerateBackgroundColor}
    color: ${({ theme }) => theme.color.white} !important;
    font-weight: 700 !important;
    font-size: 1.1rem !important;
    overflow: hidden;

    img {
        height: 50px;
        border-radius: 100%;
    }

    &:hover {
        transform: scale(1.1);
    }
`

const SFaceIcon = styled(FaceIcon)`
    width: 40px;
    height: 40px;
`

function ServerFab({ serverName, navigateOnClick, className }) {
    const { servers } = useServerAndRoomInfo()
    const { registry, registryStateURI } = useServerRegistry(serverName)
    const { navigate } = useNavigation()
    const { httpHost } = useRedwood()

    const onClick = useCallback(() => {
        if (navigateOnClick) {
            navigate(serverName, null)
        }
    }, [navigate, navigateOnClick, serverName])

    const isDirectMessage = useMemo(
        () => (servers[serverName] || {}).isDirectMessage,
        [servers, serverName],
    )

    if (isDirectMessage) {
        return (
            <SFab $isDirectMessage={isDirectMessage} onClick={onClick}>
                <SFaceIcon />
            </SFab>
        )
    }

    if (registry && registry.iconImg) {
        return (
            <SFab $text={serverName} onClick={onClick} className={className}>
                {/* NOTE: Create component to fetch server icon dynamically and check for any changes */}
                <img
                    alt="Server"
                    src={`${httpHost}/iconImg?state_uri=${registryStateURI}`}
                />
            </SFab>
        )
    }

    return (
        <SFab onClick={onClick} $text={serverName} className={className}>
            {serverName.slice(0, 1)}
        </SFab>
    )
}

export { ServerFab }
