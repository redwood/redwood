import React, { useState, useEffect } from 'react'
import styled from 'styled-components'

const STabsContainer = styled.div`
    display: flex;
    width: 100%;
    font-size: 1rem;
    justify-content: space-around;
    font-weight: 500;
    margin-bottom: 26px;
`

const STab = styled.div`
    width: 100px;
    color: black;
    border-bottom: 1px solid black;
    text-align: center;
    cursor: pointer;
`

const SActiveTab = styled.div`
    width: 100px;
    color: #ce2424;
    border-bottom: 2px solid #ce2424;
    text-align: center;
    cursor: pointer;
`

const STabsContent = styled.div`
    width: 100%;
`

const SOuterContainer = styled.div`
    width: 100%;
`

function Tabs({ tabs, initialActiveTab }) {
    const [selectedIndex, setSelectedIndex] = useState(initialActiveTab)

    useEffect(() => {
        setSelectedIndex(initialActiveTab)
    }, [initialActiveTab])

    return (
        <SOuterContainer>
            <STabsContainer>
                {tabs.map((tab, i) => (
                    selectedIndex === i
                        ? <SActiveTab key={tab.title}>{tab.title}</SActiveTab>
                        : <STab key={tab.title} onClick={() => setSelectedIndex(i)}>{tab.title}</STab>
                ))}
            </STabsContainer>
            <STabsContent>
                {tabs[selectedIndex].content}
            </STabsContent>
        </SOuterContainer>
    )
}

export default Tabs