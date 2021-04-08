import React, { useState, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import loadingSvg from '../Account/assets/loading.svg'

const LoadingIconWrapper = styled.div`
    max-width: 100px;

`

function Image({ src, className, ...props }) {
    let [ready, setReady] = useState(false)

    useEffect(() => {
        (async function () {
            while (true) {
                let resp = await fetch(src)
                if (resp.status === 200) {
                    setReady(true)
                    break
                }
                await sleep(1000)
            }
        })()
    }, [src])

    let theme = useTheme()

    if (!ready) {
        return (
            <LoadingIconWrapper>
                <svg version="1.1" id="L9" xmlns="http://www.w3.org/2000/svg" x="0px" y="0px" viewBox="0 0 100 100" enable-background="new 0 0 0 0">
                    <path fill={theme.color.grey[100]} d="M73,50c0-12.7-10.3-23-23-23S27,37.3,27,50 M30.9,50c0-10.5,8.5-19.1,19.1-19.1S69.1,39.5,69.1,50">
                        <animateTransform
                            attributeName="transform"
                            attributeType="XML"
                            type="rotate"
                            dur="1s"
                            from="0 50 50"
                            to="360 50 50"
                            repeatCount="indefinite" />
                    </path>
                </svg>
                {/*<img src={loadingSvg} className={className} {...props} />*/}
            </LoadingIconWrapper>
        )
    }
    return <img src={src} className={className} {...props} />
}

function sleep(ms) {
    return new Promise(resolve => {
        setTimeout(resolve, ms)
    })
}

export default Image
