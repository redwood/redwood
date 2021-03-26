import React, { useState, useCallback, useRef, useEffect } from 'react'
import styled from 'styled-components'

const Container = styled.div`
    position: relative;
    width: ${props => props.width}px;
    height: ${props => props.height}px;
    transition: all 0.4s cubic-bezier(0.86, 0.16, 0.16, 0.78);
    overflow: hidden;
`

const PaneWrapper = styled.div`
    position: absolute;
    top: 0;
    left: ${props => props.left}px;
    width: ${props => props.width}px;
    // transition: left 0.2s ease-in-out;
    transition: left 0.4s cubic-bezier(0.86, 0.16, 0.16, 0.78);
    display: flex;
`

export const PaneActions = styled.div`
    display: flex;
    justify-content: space-between;
    padding-top: 20px;
    flex-grow: 0;
`

export const Pane = styled.div`
    display: flex;
    flex-direction: column;
`

export const PaneContent = styled.div`
    flex-grow: 1;
`

function SlidingPane({ panes, activePane, className }) {
    let pane = panes[activePane]
    let totalWidth = panes.reduce((total, p) => total + p.width, 0)
    let left = -panes.slice(0, activePane).reduce((total, p) => total + p.width, 0)
    return (
        <Container width={pane.width} height={pane.height} className={className}>
            <PaneWrapper activePane={activePane} left={left} width={totalWidth}>
                {panes.map(p => React.cloneElement(p.content, { style: { width: p.width, height: p.height } }))}
            </PaneWrapper>
        </Container>
    )
}

export default SlidingPane