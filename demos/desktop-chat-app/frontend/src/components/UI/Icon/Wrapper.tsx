import styled from 'styled-components'
import { CSSProperties } from 'react'

export interface IconWrapperProps {
    style?: CSSProperties
    size?: number
    icon: JSX.Element
}

const SIconWrapper = styled.div<{
    size: number
}>`
    display: inline-block;
    > svg {
        font-size: ${({ size, theme }) => `${size || theme.icon.size}px`};
        fill: ${({ theme }) => theme.color.text};
    }
`

function IconWrapper({
    style = {},
    size = 0,
    icon,
}: IconWrapperProps): JSX.Element {
    return (
        <SIconWrapper style={style} size={size}>
            {icon}
        </SIconWrapper>
    )
}

export default IconWrapper
