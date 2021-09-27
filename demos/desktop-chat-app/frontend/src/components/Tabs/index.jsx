import React, { useState, useEffect } from 'react'
import styled from 'styled-components'
import { Tabs as MUITabs, Tab as MUITab } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import theme from '../../theme'

const STabsContent = styled.div`
    width: 100%;
    margin-top: 12px;
`

const SOuterContainer = styled.div`
    width: 100%;
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
    },
})

function Tabs({ tabs, initialActiveTab, className }) {
    const [selectedIndex, setSelectedIndex] = useState(initialActiveTab)
    const tabsStyles = useTabsStyles()
    const tabStyles = useTabStyles()

    useEffect(() => {
        setSelectedIndex(initialActiveTab)
    }, [initialActiveTab])

    const onChange = (x, y) => {
        setSelectedIndex(y)
    }

    return (
        <SOuterContainer className={className}>
            <MUITabs
                value={selectedIndex}
                onChange={onChange}
                variant="fullWidth"
                classes={tabsStyles}
            >
                {tabs.map((tab, i) => (
                    <MUITab
                        value={i}
                        label={tab.title}
                        key={tab.title}
                        classes={tabStyles}
                    />
                ))}
            </MUITabs>
            <STabsContent>{tabs[selectedIndex].content}</STabsContent>
        </SOuterContainer>
    )
}

export default Tabs
