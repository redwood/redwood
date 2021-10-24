import { ReactNode } from 'react'
import styled from 'styled-components'
import P from './P'

interface ErrorProps {
    children: ReactNode
}

const SError = styled.h4`
    font-family: ${({ theme }) => theme.font.type.secondary};
    color: ${({ theme }) => theme.color.secondary};
    font-size: 14px;
    text-align: center;
`

function ErrorDisplay({ children }: ErrorProps): JSX.Element {
    return <SError>{children}</SError>
}

export default ErrorDisplay
