import React, { useState, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import loadingSvg from '../Account/assets/loading.svg'

const LoadingIconWrapper = styled.div`
    max-width: 100px;
    width: 40px;
    height: 40px;
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
              <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 44 44" stroke="#fff">
                  <g fill="none" fillRule="evenodd" strokeWidth="2">
                      <circle cx="22" cy="22" r="1">
                          <animate attributeName="r" begin="0s" dur="1.8s" values="1; 20" calcMode="spline" keyTimes="0; 1" keySplines="0.165, 0.84, 0.44, 1" repeatCount="indefinite"/>
                          <animate attributeName="stroke-opacity" begin="0s" dur="1.8s" values="1; 0" calcMode="spline" keyTimes="0; 1" keySplines="0.3, 0.61, 0.355, 1" repeatCount="indefinite"/>
                      </circle>
                      <circle cx="22" cy="22" r="1">
                          <animate attributeName="r" begin="-0.9s" dur="1.8s" values="1; 20" calcMode="spline" keyTimes="0; 1" keySplines="0.165, 0.84, 0.44, 1" repeatCount="indefinite"/>
                          <animate attributeName="stroke-opacity" begin="-0.9s" dur="1.8s" values="1; 0" calcMode="spline" keyTimes="0; 1" keySplines="0.3, 0.61, 0.355, 1" repeatCount="indefinite"/>
                      </circle>
                  </g>
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
