import styled from 'styled-components'

interface ErrorOutputProps {
    children: JSX.Element
}

const SErrorOutput = styled.div`
    font-family: ${({ theme }) => theme.font.type.code};
    background: ${({ theme }) => theme.color.textDark};
    color: ${({ theme }) => theme.color.accent2};
    padding: 4px 8px;
    text-align: center;
    border: 1px solid rgba(255, 255, 255, 0.18);
    font-size: 12px;
`

function ErrorOutput({ children }: ErrorOutputProps): JSX.Element {
    return <SErrorOutput>{children}</SErrorOutput>
}

export default ErrorOutput
