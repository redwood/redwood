import { CSSProperties, useMemo } from 'react'
import styled, { css } from 'styled-components'
import { ButtonBase } from '@material-ui/core'
import { transparentize } from 'polished'

interface ButtonProps {
    style?: CSSProperties
    label?: string
    sType: string
    disabled: boolean
    icon: JSX.Element
    onClick: () => unknown
}

const disabledCss = css`
    &:disabled {
        cursor: not-allowed;
        pointer-events: auto;
        background: ${({ theme }) => theme.color.button.disabledBg};
        color: ${({ theme }) => theme.color.button.disabledColor};
        transform: translate3d(0px, 0px, 0px);
    }
`

const iconCss = css`
    border-radius: 8px;
    background: ${({ theme }) => theme.color.iconBg};
    height: 48px;
    width: 48px;
    padding: 0px;
    > svg {
        color: ${({ theme }) => theme.color.icon};
    }
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.ripple.dark};
    }
    &:hover {
        background: ${({ theme }) => theme.color.primary};
        transform: translate3d(0px, -2px, 0px);
        box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px;
        > svg {
            color: ${({ theme }) => theme.color.iconBg};
        }
    }
    &:active {
        background: ${({ theme }) => theme.color.button.primaryHover};
        box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset;
        transform: translate3d(0px, 0px, 0px);
        > svg {
            color: ${({ theme }) => theme.color.iconBg};
        }
    }
`

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
    ${disabledCss}
`

const outlineCss = css`
    background: transparent;
    color: ${({ theme }) => theme.color.text};
    border: 1px solid ${({ theme }) => theme.color.text};
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.ripple.primary};
    }
    &:hover {
        background: ${({ theme }) => transparentize(0.8, theme.color.text)};
        transform: translate3d(0px, -2px, 0px);
        box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px;
    }
    &:active {
        background: ${({ theme }) => transparentize(0.6, theme.color.text)};
        box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset;
        transform: translate3d(0px, 0px, 0px);
    }
    ${disabledCss}
`

interface SButtonProps {
    isPrimary: boolean
    isOutline: boolean
    isIcon: boolean
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
        ${({ isOutline }) => isOutline && outlineCss}
		${({ isIcon }) => isIcon && iconCss}
    }
`

function Button({
    sType,
    style,
    label = '',
    disabled,
    icon,
    onClick,
}: ButtonProps): JSX.Element {
    const isPrimary = useMemo(() => sType === 'primary', [sType])
    const isOutline = useMemo(() => sType === 'outline', [sType])
    const isIcon = useMemo(() => !!icon, [icon])

    const content = useMemo(() => {
        if (isIcon && !label) {
            return <>{icon}</>
        }

        return label
    }, [label, isIcon, icon])

    return (
        <SButton
            disabled={disabled}
            isPrimary={isPrimary}
            isOutline={isOutline}
            isIcon={isIcon}
            style={style}
            onClick={onClick}
        >
            {content}
        </SButton>
    )
}

export default Button
