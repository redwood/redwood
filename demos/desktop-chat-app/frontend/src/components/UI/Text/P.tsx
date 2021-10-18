import styled from 'styled-components'
import { CSSProperties } from 'react'

const SP = styled.p`
    font-family: ${({ theme }) => theme.font.type.primary};
    font-size: ${({ theme }) => `${theme.font.size.s2}px`};
    color: ${({ theme }) => theme.color.text};
`

const SSubP = styled.p`
    font-family: ${({ theme }) => theme.font.type.primary};
    font-size: ${({ theme }) => `${theme.font.size.s1}px`};
    color: ${({ theme }) => theme.color.text};
`

interface PProps {
    children?: string
    isSmall?: boolean
    style?: CSSProperties
    className?: string
}

function P({
    children,
    isSmall = false,
    style = {},
    className = '',
}: PProps): JSX.Element {
    if (isSmall) {
        return (
            <SSubP className={className} style={style}>
                {children}
            </SSubP>
        )
    }

    return (
        <SP className={className} style={style}>
            {children}
        </SP>
    )
}

export default P
