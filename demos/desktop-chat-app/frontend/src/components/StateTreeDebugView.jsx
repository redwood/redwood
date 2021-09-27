import React, { useState } from 'react'
import styled from 'styled-components'
import { useRedwood } from '@redwood.dev/client/react'

const SStateTreeDebugView = styled.div`
    padding: 20px;
    background-color: #d3d3d3;
    font-family: Consolas, monospace;
    font-weight: 300;
    height: calc(100vh - 90px);
    color: black;

    overflow: scroll;

    /* Chrome, Safari, Opera */
    &::-webkit-scrollbar {
        display: none;
    }
    -ms-overflow-style: none; /* IE and Edge */
    scrollbar-width: none; /* Firefox */
`

const StateURI = styled.div`
    font-weight: 700;
    cursor: pointer;
`

const SStateTree = styled(StateTree)`
    margin-bottom: 30px;
`

function StateTreeDebugView({ className }) {
    const { stateTrees } = useRedwood()
    const [disableMetadata, setDisableMetadata] = useState(true)

    const trees = {}
    for (const key of Object.keys(stateTrees)) {
        trees[key] = { ...stateTrees[key] }
    }
    if (disableMetadata) {
        for (const key of Object.keys(trees)) {
            for (const key2 of Object.keys(trees[key])) {
                if (key2 === 'Merge-Type' || key2 === 'Validator') {
                    delete trees[key][key2]
                }
            }
        }
    }

    return (
        <SStateTreeDebugView className={className}>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <h2 style={{ padding: 0 }}>State trees</h2>
                <button
                    type="button"
                    style={{ marginBottom: 32, cursor: 'pointer' }}
                    onClick={() => setDisableMetadata(!disableMetadata)}
                >
                    Toggle metadata
                </button>
            </div>
            {Object.keys(trees).map((stateURI) => (
                <SStateTree
                    tree={trees[stateURI]}
                    stateURI={stateURI}
                    key={stateURI}
                />
            ))}
        </SStateTreeDebugView>
    )
}

function StateTree({ tree, stateURI, className }) {
    const [open, setOpen] = useState(false)
    return (
        <div className={className}>
            <StateURI onClick={() => setOpen(!open)}>&gt; {stateURI}</StateURI>
            {open && (
                <div>
                    <pre>
                        <code>{JSON.stringify(tree, null, '    ')}</code>
                    </pre>
                </div>
            )}
        </div>
    )
}

export default StateTreeDebugView
