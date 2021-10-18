import { CSSProperties } from 'react'
import styled, { css, keyframes } from 'styled-components'
import { ButtonBase, Tooltip } from '@material-ui/core'

import DefaultServerAvatar from './DefaultServerAvatar'

interface ButtonProps {
    style?: CSSProperties
    serverName: string
    imageSrc?: string
    selected: boolean
    onClick: () => unknown
}

const glow = keyframes`
    0% {
        box-shadow: rgba(255, 255, 255, .25) 0 0 2px 2px;
    }
    100% {
        box-shadow: rgba(255, 255, 255, .50)  0 0 2px 0px;
    } 
`

const selectedCss = css`
    animation: ${glow} 1s alternate infinite;
    background: ${({ theme }) => theme.color.elevation1} !important;
    transform: scale(1.1);
    margin-bottom: 16px;
    margin-top: 4px;
`

const primaryCss = css<{ selected: boolean }>`
    display: flex;
    align-items: center;
    border-radius: 8px;
    background: ${({ theme }) => theme.color.iconBg};
    height: 48px;
    width: 48px;
    color: ${({ theme }) => theme.color.icon};
    margin-top: 0px;
    .MuiTouchRipple-child {
        background-color: ${({ theme }) => theme.color.ripple.dark};
    }
    &:hover {
        /* background: ${({ theme }) => theme.color.accent1}; */
        background: #3d3c3e;
        transform: ${({ selected }) =>
            selected ? 'scale(1.075)' : 'translate3d(0px, -2px, 0px)'};
        box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px;
        color: ${({ theme }) => theme.color.iconBg};
    }
    &:active {
        /* background: ${({ theme }) => theme.color.accent2}; */
        background: #5b5a62;
        box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset;
        transform: ${({ selected }) =>
            selected ? 'scale(1.075)' : 'translate3d(0px, 0px, 0px)'};
    }
    ${({ selected }) => selected && selectedCss};
`

const SButton = styled(ButtonBase)<{ selected: boolean }>`
    &&& {
        display: inline-block;
        white-space: nowrap;
        font-family: ${({ theme }) => theme.font.type.primary};
        border: none;
        padding: 8px 16px;
        border-radius: 2px;
        font-size: ${({ theme }) => `${theme.font.size.s2}px`};
        transition: margin-top ease-in-out 250ms,
            margin-bottom ease-in-out 250ms,
            ${({ theme }) => theme.transition.cubicBezier};
        transform: translate3d(0px, 0px, 0px);

        ${primaryCss}
    }
`

function Button({
    style,
    onClick,
    serverName,
    selected,
    imageSrc,
}: ButtonProps): JSX.Element {
    const content = <DefaultServerAvatar serverName={serverName} />

    return (
        <Tooltip title={serverName} placement="right" arrow>
            <SButton selected={selected} style={style} onClick={onClick}>
                {content}
            </SButton>
        </Tooltip>
    )
}

export default Button
