import styled from 'styled-components'
import { CSSProperties } from 'react'

const SHeader = styled.h1`
    font-family: ${({ theme }) => theme.font.type.secondary};
    font-size: ${({ theme }) => `${theme.font.size.l2}px`};
    color: ${({ theme }) => theme.color.text};
`

const SSubHeader = styled.h1`
    font-family: ${({ theme }) => theme.font.type.secondary};
    font-size: ${({ theme }) => `${theme.font.size.m3}px`};
    color: ${({ theme }) => theme.color.text};
`

interface HeaderProps {
    children: string
    isSmall: boolean
    style: CSSProperties
}

function Header({
    children = '',
    isSmall = false,
    style = {},
}: HeaderProps): JSX.Element {
    if (isSmall) {
        return <SSubHeader style={style}>{children}</SSubHeader>
    }

    return <SHeader style={style}>{children}</SHeader>
}

export default Header
