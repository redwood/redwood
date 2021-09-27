import React from 'react'
import styled from 'styled-components'
import { parser as cssMath } from 'css-math'

const Container = styled.div`
    position: relative;
    width: ${(props) => props.width};
    height: ${(props) => (props.height ? props.height : 'unset')};
    transition: all 0.4s cubic-bezier(0.86, 0.16, 0.16, 0.78);
    overflow: hidden;
`

const PaneWrapper = styled.div`
    position: absolute;
    top: 0;
    left: ${(props) => props.left};
    width: ${(props) => props.width};
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
    const pane = panes[activePane]
    const totalWidth = panes.reduce(
        (total, p) => cssMath(`${total} + ${p.width}`),
        '0px',
    )
    const left = panes
        .slice(0, activePane)
        .reduce((total, p) => cssMath(`${total} - ${p.width}`), '0px')
    return (
        <Container
            width={pane.width}
            height={pane.height}
            minHeight={pane.minHeight}
            maxHeight={pane.maxHeight}
            className={className}
        >
            <PaneWrapper activePane={activePane} left={left} width={totalWidth}>
                {panes.map((p) =>
                    React.cloneElement(p.content, {
                        style: { width: p.width, height: p.height },
                    }),
                )}
            </PaneWrapper>
        </Container>
    )
}

export default SlidingPane
