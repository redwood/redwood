import React from 'react'
import { Tooltip as MUITooltip } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'

const useStylesBootstrap = makeStyles(() => ({
    arrow: {
        color: 'black',
    },
    tooltip: {
        backgroundColor: 'black',
        fontSize: '0.9rem',
        fontFamily: 'Noto Sans KR',
    },
}))

function Tooltip(props) {
    const classes = useStylesBootstrap()
    return <MUITooltip arrow classes={classes} {...props} />
}

export default Tooltip
