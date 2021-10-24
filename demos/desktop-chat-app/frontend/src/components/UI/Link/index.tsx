import { CSSProperties, ReactNode } from 'react'
import styled from 'styled-components'
import { Link as RouterLink } from 'react-router-dom'
import { darken } from 'polished'

interface LinkProps {
    className?: string
    style?: CSSProperties
    linkTo?: string
    children: ReactNode
}

const SRouterLink = styled(RouterLink)`
    color: ${({ theme }) => theme.color.primary};
    font-family: ${({ theme }) => theme.font.type.primary};
    font-size: 12px;
    text-decoration: none;
    transition: ${({ theme }) => theme.transition.primary};
    &:hover {
        color: ${({ theme }) => darken(0.18, theme.color.primary)};
    }
`

function Link({
    style = {},
    className = '',
    linkTo = '',
    children,
}: LinkProps): JSX.Element {
    return (
        <SRouterLink style={style} className={className} to={linkTo}>
            {children}
        </SRouterLink>
    )
}

export default Link
