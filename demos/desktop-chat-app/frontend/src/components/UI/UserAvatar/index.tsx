import { CSSProperties, useMemo } from 'react'
import styled, { css } from 'styled-components'
import { Avatar } from '@material-ui/core'
import { desaturate, darken } from 'polished'
import RetryIcon from '@material-ui/icons/Replay'

import DefaultAvatar from './DefaultAvatar'
import strToColor from '../../../utils/strToColor'
import loadingSvg from './assets/loading.svg'

const generateBgColor = (text: string): string =>
    desaturate(0.25, darken(0.1, strToColor(text)))

interface UserAvatarProps {
    style?: CSSProperties
    address: string
    username?: string
    loadingImage?: boolean
    circle?: boolean
    large?: boolean
    imageSrc?: string
    loadFailed?: boolean
    onRetry?: () => unknown
}

const retryCss = css<{ large: boolean }>`
    background: ${({ theme }) => theme.color.accent1};
    cursor: pointer;
    transition: ${({ theme }) => theme.transition.primary};
    > svg {
        color: ${({ theme }) => theme.color.primary};
        width: ${({ large }) => (large ? '50px' : '24px')};
        height: ${({ large }) => (large ? '50px' : '24px')};
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
`

const SUserAvatar = styled.div<{
    bgColor: string
    circle: boolean
    large: boolean
    loadFailed?: boolean
}>`
    position: relative;
    height: 40px;
    width: 40px;
    color: white;
    border-radius: ${({ circle }) => (circle ? '100%' : '4px')};
    background: ${({ bgColor }) => bgColor};
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 8px;
    .loading-avatar-placeholder {
        height: ${({ large }) => (large ? '48px' : '18px')} !important;
    }
    ${({ large }) =>
        large &&
        css`
            height: 100px;
            width: 100px;
            > img {
                height: 72px;
            }
        `}
    ${({ loadFailed }) => loadFailed && retryCss};
`

const SImageAvatar = styled(Avatar)<{ large: boolean }>`
    margin: 8px;
    ${({ large }) =>
        large &&
        css`
            &&& {
                height: 100px !important;
                width: 100px !important;
            }
        `}
`

const SLoadingOverlay = styled.div<{ large: boolean; circle: boolean }>`
    height: 100%;
    width: 100%;
    background: rgba(14, 12, 44, 0.4);
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: ${({ circle }) => (circle ? '100%' : '4px')};
    .loading-avatar-image {
        height: ${({ large }) => (large ? '90px' : '40px')};
        transform: scale(1.6);
        pointer-events: none;
        user-select: none;
    }
`

function UserAvatar({
    style = {},
    address = '',
    username = '',
    imageSrc = '',
    loadingImage = false,
    large = false,
    circle = false,
    loadFailed = false,
    onRetry = () => false,
}: UserAvatarProps): JSX.Element {
    const bgColor = useMemo(() => generateBgColor(address), [address])

    if (imageSrc) {
        if (loadingImage) {
            return (
                <SUserAvatar
                    large={large}
                    circle={circle}
                    bgColor={bgColor}
                    style={style}
                >
                    <SLoadingOverlay circle={circle} large={large}>
                        <img
                            className="loading-avatar-image"
                            alt="Loading Img"
                            src={loadingSvg}
                        />
                    </SLoadingOverlay>
                    <DefaultAvatar
                        className="loading-avatar-placeholder"
                        address={address}
                    />
                </SUserAvatar>
            )
        }

        if (loadFailed) {
            return (
                <SUserAvatar
                    large={large}
                    circle={circle}
                    bgColor={bgColor}
                    style={style}
                    loadFailed={loadFailed}
                    onClick={onRetry}
                >
                    <RetryIcon />
                </SUserAvatar>
            )
        }

        return (
            <SImageAvatar
                variant={circle ? 'circle' : 'rounded'}
                alt="Avatar Img"
                src={imageSrc}
                large={large}
            />
        )
    }

    return (
        <SUserAvatar
            large={large}
            circle={circle}
            bgColor={bgColor}
            style={style}
        >
            <DefaultAvatar address={address} />
        </SUserAvatar>
    )
}

export default UserAvatar
