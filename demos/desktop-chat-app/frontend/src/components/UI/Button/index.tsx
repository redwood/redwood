import { CSSProperties, useMemo } from 'react'
import styled, { css } from 'styled-components'
import { ButtonBase } from '@material-ui/core'
import { transparentize } from 'polished'

interface ButtonProps {
    style?: CSSProperties
    label?: string
    sType: string
    disabled?: boolean
    icon?: JSX.Element
    flipIcon?: boolean
    className?: string
    onClick: () => unknown
}

const disabledCss = css`
    &:disabled {
        cursor: not-allowed;
        pointer-events: auto;
        background: ${({ theme }) => theme.color.button.disabledBg};
        color: ${({ theme }) => theme.color.button.disabledColor};
        transform: translate3d(0px, 0px, 0px);
        box-shadow: none;
    }
`

const iconCss = css<{ hasLabel: boolean; flipIcon: boolean }>`
    display: flex;
    align-items: center;
    flex-direction: ${({ flipIcon }) => (flipIcon ? 'row-reverse' : 'row')};
    border-radius: 8px;
    background: ${({ theme }) => theme.color.iconBg};
    height: ${({ hasLabel }) => (hasLabel ? '36px' : '48px')};
    width: ${({ hasLabel }) => (hasLabel ? 'auto' : '48px')};
    padding: ${({ hasLabel }) => (hasLabel ? '0px 12px' : '0px')};
    color: ${({ theme }) => theme.color.icon};
    > svg {
        color: ${({ theme }) => theme.color.icon};
        padding-right: ${({ hasLabel, flipIcon }) =>
            hasLabel && flipIcon ? '6px' : '0px'};
        padding-left: ${({ hasLabel, flipIcon }) =>
            hasLabel && !flipIcon ? '6px' : '0px'};
        font-size: ${({ hasLabel }) => (hasLabel ? '20px' : '24px')};
    }
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.ripple.dark};
    }
    &:hover {
        background: ${({ theme }) => theme.color.primary};
        transform: translate3d(0px, -2px, 0px);
        box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px;
        color: ${({ theme }) => theme.color.iconBg};
        > svg {
            color: ${({ theme }) => theme.color.iconBg};
        }
    }
    &:active {
        background: ${({ theme }) => theme.color.button.primaryHover};
        box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset;
        transform: translate3d(0px, 0px, 0px);
        color: ${({ theme }) => theme.color.iconBg};
        > svg {
            color: ${({ theme }) => theme.color.iconBg};
        }
    }
    ${disabledCss}
    &:disabled {
        > svg {
            color: ${({ theme }) => theme.color.button.disabledColor};
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
    &:disabled {
        border-color: ${({ theme }) => theme.color.button.disabledBg};
    }
`

interface SButtonProps {
    isPrimary: boolean
    isOutline: boolean
    hasIcon: boolean
    hasLabel: boolean
    flipIcon: boolean
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

        ${({ isPrimary, hasIcon }) => !hasIcon && isPrimary && primaryCss}
        ${({ isOutline, hasIcon }) => !hasIcon && isOutline && outlineCss}
		${({ hasIcon }) => hasIcon && iconCss}
    }
`

function Button({
    sType,
    style,
    label = '',
    disabled,
    icon,
    flipIcon,
    className = '',
    onClick,
}: ButtonProps): JSX.Element {
    const isPrimary = useMemo(() => sType === 'primary', [sType])
    const isOutline = useMemo(() => sType === 'outline', [sType])
    const hasIcon = useMemo(() => !!icon, [icon])
    const hasLabel = useMemo(() => !!label, [label])

    const content = useMemo(() => {
        if (hasIcon && !hasLabel) {
            return <>{icon}</>
        }
        if (hasIcon && hasLabel) {
            return (
                <>
                    {label} {icon}
                </>
            )
        }

        return label
    }, [hasLabel, hasIcon, icon, label])
    return (
        <SButton
            className={className}
            disabled={disabled}
            isPrimary={isPrimary}
            isOutline={isOutline}
            hasIcon={hasIcon}
            flipIcon={!!flipIcon}
            hasLabel={hasLabel}
            style={style}
            onClick={onClick}
        >
            {content}
        </SButton>
    )
}

export default Button
