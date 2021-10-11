import styled from 'styled-components'
import { CSSProperties } from 'react'
import { ButtonBase } from '@material-ui/core'
import { darken, adjustHue } from 'polished'

interface ButtonProps {
    style?: CSSProperties
    label?: string
    onClick: () => unknown
}

const SButton = styled(ButtonBase)`
    &&& {
        background: ${({ theme }) => theme.color.primary};
        border: none;
        padding: 8px 16px;
        border-radius: 2px;
        color: ${({ theme }) => theme.color.textDark};
        font-family: ${({ theme }) => theme.font.type.primary};
        font-size: ${({ theme }) => `${theme.font.size.s2}px`};
        transition: ${({ theme }) => theme.transition.primary};
        .MuiTouchRipple-child {
            background-color: ${({ theme }) => theme.color.ripple.secondary};
        }
        &:hover {
            background: ${({ theme }) =>
                adjustHue(30, darken(0.2, theme.color.primary))};
        }
    }
`

function Button({ style, label = '', onClick }: ButtonProps): JSX.Element {
    return <SButton style={style}>{label}</SButton>
}

export default Button
