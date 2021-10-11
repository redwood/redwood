import { CSSProperties, useMemo } from 'react'
import styled, { css } from 'styled-components'
import { ButtonBase } from '@material-ui/core'

interface ButtonProps {
    style?: CSSProperties
    label?: string
    sType: string
    disabled: boolean
    onClick: () => unknown
}

const primaryCss = css`
    background: ${({ theme }) => theme.color.primary};
    color: ${({ theme }) => theme.color.textDark};
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.ripple.primary};
    }
    &:hover {
        background: ${({ theme }) => theme.color.button.primaryHover};
        transform: translate3d(0px, -2px, 0px);
        box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px;
    }
    &:active {
        background: ${({ theme }) => theme.color.button.primaryActive};
        box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset;
        transform: translate3d(0px, 0px, 0px);
    }
    &:disabled {
        cursor: not-allowed;
        pointer-events: auto;
        background: ${({ theme }) => theme.color.button.disabledBg};
        color: ${({ theme }) => theme.color.button.disabledColor};
        transform: translate3d(0px, 0px, 0px);
    }
`

interface SButtonProps {
    isPrimary: boolean
}

const SButton = styled(ButtonBase)<SButtonProps>`
    &&& {
        display: inline-block;
        white-space: nowrap;
        font-family: ${({ theme }) => theme.font.type.primary};
        border: none;
        padding: 8px 16px;
        border-radius: 2px;
        font-size: ${({ theme }) => `${theme.font.size.s2}px`};
        transition: ${({ theme }) => theme.transition.cubicBezier};
        transform: translate3d(0px, 0px, 0px);

        ${({ isPrimary }) => isPrimary && primaryCss}
    }
`

function Button({
    sType,
    style,
    label = '',
    disabled,
    onClick,
}: ButtonProps): JSX.Element {
    const isPrimary = useMemo(() => sType === 'primary', [sType])

    return (
        <SButton
            disabled={disabled}
            isPrimary={isPrimary}
            style={style}
            onClick={onClick}
        >
            {label}
        </SButton>
    )
}

export default Button
