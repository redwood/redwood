import React, { useState } from 'react'
import styled from 'styled-components'
import { useRedwood } from 'redwood-p2p-client/react'
// import Button from '../Button'

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
    const [disableMetadata, setDisableMetadata] = useState(true)

    let trees = {}
    for (let key of Object.keys(stateTrees)) {
        trees[key] = { ...stateTrees[key] }
    }
    if (disableMetadata) {
        for (let key of Object.keys(trees)) {
            for (let key2 of Object.keys(trees[key])) {
                if (key2 === 'Merge-Type' || key2 === 'Validator') {
                    delete trees[key][key2]
                }
            }
        }
    }

    return null

    return (
        <SStateTreeDebugView className={className}>
            <button onClick={() => setDisableMetadata(!disableMetadata)}>Toggle metadata</button>
            {Object.keys(trees).map(stateURI => (
                <StateTree key={stateURI}>
                    <StateURI>&gt; {stateURI}</StateURI>
                    <pre><code>{JSON.stringify(trees[stateURI], null, '    ')}</code></pre>
                </StateTree>
            ))}
        </SStateTreeDebugView>
    )
}

export default StateTreeDebugView
