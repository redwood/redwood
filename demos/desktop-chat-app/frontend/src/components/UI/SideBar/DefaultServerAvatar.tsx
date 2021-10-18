import { useMemo } from 'react'
import styled from 'styled-components'
import { createAvatar } from '@dicebear/avatars'
import * as style from '@dicebear/avatars-jdenticon-sprites'

interface DefaultServerAvatarProps {
    serverName: string
}

const SDefaultServerAvatar = styled.img`
    height: 36px;
`

function DefaultServerAvatar({
    serverName,
}: DefaultServerAvatarProps): JSX.Element {
    const defaultServerAvatar = useMemo(
        () =>
            createAvatar(style, {
                seed: serverName,
                base64: true,
                // colors: [
                //     'amber',
                //     'blue',
                //     'brown',
                //     'cyan',
                //     'deepOrange',
                //     'deepPurple',
                //     'green',
                //     'indigo',
                //     'lightBlue',
                //     'lightGreen',
                //     'lime',
                //     'orange',
                //     'pink',
                //     'purple',
                //     'red',
                //     'teal',
                //     'yellow',
                // ],
                // colorLevel: 500,
            }),
        [serverName],
    )

    return <SDefaultServerAvatar alt={serverName} src={defaultServerAvatar} />
}

export default DefaultServerAvatar
