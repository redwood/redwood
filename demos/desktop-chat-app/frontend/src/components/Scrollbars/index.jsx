import React, { useState, useCallback, useEffect, useRef } from 'react'
import styled from 'styled-components'
import { Scrollbars } from 'react-custom-scrollbars'

const SScrollbars = styled.div`
    & {
        position: relative;
        overflow: hidden;
    }

    &:before {
        content: "";
        width: 100%;
        height: 23px;
        display: block;
        position: absolute;
        top: 0;
        left: -15%;
        box-shadow: inset 0px 10px 11px -2px rgba(0,0,0, 0.3);
        transition: opacity 0.05s;
        width: 130%;
        opacity: ${props => props.showTopShadow ? 100 : 0}%;
        z-index: 9999;
    }

    &:after {
        content: "";
        width: 100%;
        height: 23px;
        display: block;
        position: absolute;
        bottom: 0;
        left: -15%;
        box-shadow: inset 0px -10px 11px -2px rgba(0,0,0, 0.3);
        transition: opacity 0.05s;
        width: 130%;
        opacity: ${props => props.showBottomShadow ? 100 : 0}%;
        z-index: 9999;
    }
`

export default function({ shadow, autoHeight, autoHeightMin, autoHeightMax, className, style, innerRef, children, ...props }) {
    const [showTopShadow, setShowTopShadow] = useState(false)
    const [showBottomShadow, setShowBottomShadow] = useState(false)

    const onUpdate = useCallback(values => {
        const { scrollTop, scrollHeight, clientHeight } = values
        const bottomScrollTop = scrollHeight - clientHeight

        // No scrolling needed, no shadow
        if (bottomScrollTop === 0) {
            setShowTopShadow(false)
            setShowBottomShadow(false)
            return
        }

        let scrollPercent = scrollTop / bottomScrollTop
        setShowTopShadow(scrollPercent >= 0.1)
        setShowBottomShadow(scrollPercent <= 0.9)
    }, [ setShowTopShadow, setShowBottomShadow ])

    if (shadow) {
        return (
            <SScrollbars
                className={className}
                style={style}
                showTopShadow={showTopShadow}
                showBottomShadow={showBottomShadow}
                {...props}
            >
                <Scrollbars
                    ref={innerRef}
                    autoHide
                    autoHideTimeout={1000}
                    autoHideDuration={200}
                    autoHeight={autoHeight}
                    autoHeightMin={autoHeightMin}
                    autoHeightMax={autoHeightMax}
                    onUpdate={onUpdate}
                >
                    {children}
                </Scrollbars>
            </SScrollbars>
        )
    } else {
        return (
            <Scrollbars
                ref={innerRef}
                autoHide
                autoHideTimeout={1000}
                autoHideDuration={200}
                autoHeight={autoHeight}
                autoHeightMin={autoHeightMin}
                autoHeightMax={autoHeightMax}
                {...props}
            >
                {children}
            </Scrollbars>
        )
    }
}
        {/*<div style={{ display: 'flex', flexDirection: 'column', flexGrow: 1 }}>
            <ShadowTop opacity={shadowTopOpacity} />*/}
            {/*<ShadowBottom opacity={shadowBottomOpacity} />
        </div>*/}

