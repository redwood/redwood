import { useMemo } from 'react'
import styled from 'styled-components'
import { createAvatar } from '@dicebear/avatars'
import * as style from '@dicebear/avatars-bottts-sprites'

interface DefaultAvatarProps {
    address: string
    className?: string
}

const SDefaultAvatar = styled.img`
    height: 32px;
`

function DefaultAvatar({
    address,
    className = '',
}: DefaultAvatarProps): JSX.Element {
    const defaultAvatar = useMemo(
        () =>
            createAvatar(style, {
                seed: address,
                base64: true,
                colorful: true,
            }),
        [address],
    )

    return (
        <SDefaultAvatar
            alt="Avatar"
            className={className}
            src={defaultAvatar}
        />
    )
}

export default DefaultAvatar
