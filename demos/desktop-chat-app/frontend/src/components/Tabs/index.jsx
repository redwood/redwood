import React, { useState, useEffect } from 'react'
import styled, { useTheme } from 'styled-components'
import { Tabs as MUITabs, Tab as MUITab } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import theme from '../../theme'

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
    color: ${props => props.theme.color.grey[50]};
    border-bottom: 1px solid ${props => props.theme.color.grey[50]};
    text-align: center;
    cursor: pointer;
    transition: all 100ms ease-in-out;

    &.active {
        width: 100px;
        color: ${props => props.theme.color.green[500]};
        border-bottom: 2px solid ${props => props.theme.color.green[500]};
        text-align: center;
        cursor: pointer;
        transition: all 100ms ease-in-out;
    }
`

// const SActiveTab = styled.div`
//     width: 100px;
//     color: ${props => props.theme.color.green[500]};
//     border-bottom: 2px solid ${props => props.theme.color.green[500]};
//     text-align: center;
//     cursor: pointer;
//     transition: color 500ms ease-in-out;
// `

const STabsContent = styled.div`
    width: 100%;
    margin-top: 12px;
`

const SOuterContainer = styled.div`
    width: 100%;
`

const SMUITabs = styled(MUITabs)`
    border-bottom: 1px solid ${props => props.theme.color.grey[100]};
`

const useTabsStyles = makeStyles({
    indicator: {
        backgroundColor: theme.color.green[500],
    },
})

const useTabStyles = makeStyles({
    root: {
        borderTopLeftRadius: 16,
        borderTopRightRadius: 16,
        borderBottom: `1px solid ${theme.color.grey[200]}`,
        fontFamily: 'Noto Sans KR',
        textTransform: 'none',
    }
})

function Tabs({ tabs, initialActiveTab, className }) {
    const [selectedIndex, setSelectedIndex] = useState(initialActiveTab)
    const theme = useTheme()
    const tabsStyles = useTabsStyles()
    const tabStyles = useTabStyles()

    useEffect(() => {
        setSelectedIndex(initialActiveTab)
    }, [initialActiveTab])

    function onChange(x, y) {
        setSelectedIndex(y)
    }

    return (
        <SOuterContainer className={className}>
            <MUITabs value={selectedIndex} onChange={onChange} variant="fullWidth" classes={tabsStyles}>
                {tabs.map((tab, i) => (
                    <MUITab value={i} label={tab.title} key={tab.title} classes={tabStyles} />
                ))}
            </MUITabs>
            <STabsContent>
                {tabs[selectedIndex].content}
            </STabsContent>
            {/*<STabsContainer>
                {tabs.map((tab, i) => (
                    <STab className={selectedIndex === i ? 'active' : null} key={tab.title} onClick={() => setSelectedIndex(i)}>{tab.title}</STab>
                ))}
            </STabsContainer>
            <STabsContent>
                {tabs[selectedIndex].content}
            </STabsContent>*/}
        </SOuterContainer>
    )
}

export default Tabs