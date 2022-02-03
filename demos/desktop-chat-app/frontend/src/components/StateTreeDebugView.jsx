import React, { Fragment, useState } from 'react'
import styled from 'styled-components'
import { useRedwood } from '@redwood.dev/client/react'
import theme from '../theme'
import Scrollbars from './Scrollbars'

const SStateTreeDebugView = styled.div`
    font-family: Consolas, monospace;
    font-weight: 300;
    color: rgba(255,255,255,0.8);
    background-color: ${_ => theme.color.grey[200]};
    border-left: 2px solid ${_ => theme.color.grey[300]};
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

    return (
        <SStateTreeDebugView className={className}>
            <Scrollbars shadow style={{ height: '100%' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', padding: '20px 20px 0 20px' }}>
                    <button style={{ marginBottom: 32, cursor: 'pointer' }} onClick={() => setDisableMetadata(!disableMetadata)}>Toggle metadata</button>
                </div>
                <div style={{ padding: '0 20px 20px 20px' }}>
                    {Object.keys(trees).map(stateURI => (
                        <SStateTree tree={trees[stateURI]} stateURI={stateURI} key={stateURI} />
                    ))}
                </div>
            </Scrollbars>
        </SStateTreeDebugView>
    )
}

function StateTree({ tree, stateURI, className }) {
    let [open, setOpen] = useState(false)
    return (
        <div className={className}>
            <StateURI onClick={() => setOpen(!open)}>&gt; {stateURI}</StateURI>
            {open && <div>
                <pre><code>{JSON.stringify(tree, null, '    ')}</code></pre>
            </div>}
        </div>
    )
}

export default StateTreeDebugView
