import styled from 'styled-components'
import { CSSProperties } from 'react'

const SP = styled.h1`
    font-family ${({ theme }) => theme.font.type.primary};
    font-size: ${({ theme }) => `${theme.font.size.s2}px`};
    color: ${({ theme }) => theme.color.text};
`

const SSubP = styled.h1`
    font-family ${({ theme }) => theme.font.type.primary};
    font-size: ${({ theme }) => `${theme.font.size.s1}px`};
    color: ${({ theme }) => theme.color.text};
`

interface PProps {
    children?: string
    isSmall?: boolean
    style?: CSSProperties
}

function P({ children, isSmall = false, style = {} }: PProps): JSX.Element {
    if (isSmall) {
        return <SSubP style={style}>{children}</SSubP>
    }

    return <SP style={style}>{children}</SP>
}

export default P
