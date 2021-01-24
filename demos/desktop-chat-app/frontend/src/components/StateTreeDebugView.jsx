import React, { useState } from 'react'
import styled from 'styled-components'
import useRedwood from '../hooks/useRedwood'

const SStateTreeDebugView = styled.div`
    padding: 20px;
    background-color: #d3d3d3;
    font-family: Consolas, monospace;
    font-weight: 300;
    height: calc(100vh - 40px);
    color: black;

    overflow: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none;  /* IE and Edge */
    scrollbar-width: none;  /* Firefox */
`

const StateURI = styled.div`
    font-weight: 700;
`

const StateTree = styled.div`
    margin-bottom: 30px;
`

function StateTreeDebugView({ className }) {
    const { stateTrees } = useRedwood()

    return null

    return (
        <SStateTreeDebugView className={className}>
            {Object.keys(stateTrees).map(stateURI => (
                <StateTree key={stateURI}>
                    <StateURI>&gt; {stateURI}</StateURI>
                    <pre><code>{JSON.stringify(stateTrees[stateURI], null, '    ')}</code></pre>
                </StateTree>
            ))}
        </SStateTreeDebugView>
    )
}

export default StateTreeDebugView
