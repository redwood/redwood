import { Tooltip as MUITooltip } from '@material-ui/core'
import { withStyles, makeStyles } from '@material-ui/core/styles'

const useStylesBootstrap = makeStyles((theme) => ({
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
