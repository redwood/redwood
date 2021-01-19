import React, { useState } from 'react'
import styled from 'styled-components'
import useBraid from '../hooks/useBraid'

const SStateTreeDebugView = styled.div`
    padding: 20px;
    background-color: #d3d3d3;
    font-family: Consolas, monospace;
    font-weight: 300;
    height: 100vh;

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
    const { appState, nodeAddress, registry } = useBraid()

    return (
        <SStateTreeDebugView className={className}>
            <StateTree>
                <StateURI>&gt; chat.redwood.dev/registry</StateURI>
                <pre><code>{JSON.stringify(registry, null, '    ')}</code></pre>
            </StateTree>

            {Object.keys(appState).map(stateURI => (
                <StateTree key={stateURI}>
                    <StateURI>&gt; {stateURI}</StateURI>
                    <pre><code>{JSON.stringify(appState[stateURI], null, '    ')}</code></pre>
                </StateTree>
            ))}
        </SStateTreeDebugView>
    )
}

export default StateTreeDebugView
