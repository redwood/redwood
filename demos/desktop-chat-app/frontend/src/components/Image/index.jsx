import React, { useState, useEffect, useCallback } from 'react'
import styled from 'styled-components'
import useNavigation from '../../hooks/useNavigation'
import gooLoading from '../Account/assets/loading-goo.svg'
import failedSvg from '../../assets/failed-image.svg'
import Button from '../Button'

const LoadingIconWrapper = styled.div`
    max-width: 100px;
    width: 40px;
    height: 40px;
`

const FailedImageWrapper = styled.div`
    display: flex;
    flex-direction: column;
    width: 120px;
    > span {
        font-size: 12px;
        color: #ff3e3e;
        text-align: center;
        margin-bottom: 8px;
        font-weight: 500;
    }
    img {
        height: 72px;
    }
`

function Image({
    src,
    className,
    loadFailed,
    setLoadFailed,
    onClick,
    ...props
}) {
    const [ready, setReady] = useState(true)
    const { selectedStateURI } = useNavigation()

    const fetchImage = useCallback(async () => {
        let failedIntervals = 0
        let cancelRequest = false
        /* eslint-disable no-await-in-loop */
        while (true) {
            if (cancelRequest) {
                break
            }
            const resp = await fetch(src)
            if (resp.status === 200) {
                failedIntervals = 0
                setReady(true)
                break
            }
            if (failedIntervals >= 7) {
                setLoadFailed(true)
                break
            }
            await sleep(1000)
            failedIntervals += 1
        }
        /* eslint-enable no-await-in-loop */

        return () => {
            cancelRequest = true
        }
    }, [selectedStateURI])

    useEffect(() => {
        if (loadFailed === 'retry') {
            setLoadFailed(false)
            fetchImage()
        }
    }, [loadFailed])

    useEffect(fetchImage, [src])

    if (loadFailed === 'failed') {
        return (
            <FailedImageWrapper>
                <span>Image Failed to Load</span>
                <img alt="Failed" src={failedSvg} />
                <Button
                    primary
                    style={{
                        width: '100%',
                        marginTop: 12,
                        fontSize: 12,
                        padding: '3px 6px',
                        lineHeight: '1.25',
                    }}
                    onClick={() => setLoadFailed('retry')}
                >
                    Retry Image
                </Button>
            </FailedImageWrapper>
        )
    }

    if (!ready) {
        // NOTE: Need to handle this in a dynamtic way
        if (props.mentioninfocard) {
            return (
                <LoadingIconWrapper style={props.style || {}}>
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="16"
                        height="16"
                        viewBox="0 0 18 18"
                        stroke="#fff"
                    >
                        <g fill="none" fillRule="evenodd" strokeWidth="2">
                            <circle cx="8" cy="8" r="1">
                                <animate
                                    attributeName="r"
                                    begin="0s"
                                    dur="1.8s"
                                    values="1; 7.5"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.165, 0.84, 0.44, 1"
                                    repeatCount="indefinite"
                                />
                                <animate
                                    attributeName="stroke-opacity"
                                    begin="0s"
                                    dur="1.8s"
                                    values="1; 0"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.3, 0.61, 0.355, 1"
                                    repeatCount="indefinite"
                                />
                            </circle>
                            <circle cx="8" cy="8" r="1">
                                <animate
                                    attributeName="r"
                                    begin="-0.9s"
                                    dur="1.8s"
                                    values="1; 7.5"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.165, 0.84, 0.44, 1"
                                    repeatCount="indefinite"
                                />
                                <animate
                                    attributeName="stroke-opacity"
                                    begin="-0.9s"
                                    dur="1.8s"
                                    values="1; 0"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.3, 0.61, 0.355, 1"
                                    repeatCount="indefinite"
                                />
                            </circle>
                        </g>
                    </svg>
                </LoadingIconWrapper>
            )
        }

        if (props.mentionsuggestion) {
            return (
                <LoadingIconWrapper style={props.style || {}}>
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        viewBox="0 0 22 22"
                        stroke="#fff"
                    >
                        <g fill="none" fillRule="evenodd" strokeWidth="2">
                            <circle cx="11" cy="11" r="1">
                                <animate
                                    attributeName="r"
                                    begin="0s"
                                    dur="1.8s"
                                    values="1; 10"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.165, 0.84, 0.44, 1"
                                    repeatCount="indefinite"
                                />
                                <animate
                                    attributeName="stroke-opacity"
                                    begin="0s"
                                    dur="1.8s"
                                    values="1; 0"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.3, 0.61, 0.355, 1"
                                    repeatCount="indefinite"
                                />
                            </circle>
                            <circle cx="11" cy="11" r="1">
                                <animate
                                    attributeName="r"
                                    begin="-0.9s"
                                    dur="1.8s"
                                    values="1; 10"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.165, 0.84, 0.44, 1"
                                    repeatCount="indefinite"
                                />
                                <animate
                                    attributeName="stroke-opacity"
                                    begin="-0.9s"
                                    dur="1.8s"
                                    values="1; 0"
                                    calcMode="spline"
                                    keyTimes="0; 1"
                                    keySplines="0.3, 0.61, 0.355, 1"
                                    repeatCount="indefinite"
                                />
                            </circle>
                        </g>
                    </svg>
                </LoadingIconWrapper>
            )
        }
        return (
            <LoadingIconWrapper
                style={
                    props.style || {
                        height: 120,
                        width: 120,
                    }
                }
            >
                <img
                    alt="Loading Icon"
                    src={gooLoading}
                    style={{
                        height: 120,
                        width: 120,
                        background: '#2a2d32',
                        borderRadius: 4,
                        border: '1px solid rgba(255, 255, 255, .2)',
                    }}
                />
            </LoadingIconWrapper>
        )
    }

    return (
        <img
            style={{ cursor: onClick ? 'pointer' : 'default' }}
            onClick={onClick}
            src={src}
            className={className}
            alt="Preview"
            {...props}
        />
    )
}

function sleep(ms) {
    return new Promise((resolve) => {
        setTimeout(resolve, ms)
    })
}

export default Image
