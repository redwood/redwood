import { useContext } from 'react'
import styled, { ThemeContext } from 'styled-components'

function Spacer({ size = 'md' }) {
    const { spacing } = useContext(ThemeContext)

    let s
    switch (size) {
        case 'lg':
            s = spacing[6]
            break
        case 'sm':
            s = spacing[2]
            break
        case 'md':
        default:
            s = spacing[4]
    }

    if (size === 'flex') {
        return <div style={{ flexGrow: 1 }} />
    }
    return <StyledSpacer size={s} />
}

const StyledSpacer = styled.div`
    height: ${(props) => props.size}px;
    width: ${(props) => props.size}px;
`

export default Spacer
