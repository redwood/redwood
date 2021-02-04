import React, { useState } from 'react'
import styled from 'styled-components'
import { withStyles } from '@material-ui/core/styles'
import { Button as MUIButton } from '@material-ui/core'
import theme from '../../theme'

const PrimaryButton = withStyles({
    root: {
        backgroundColor: theme.color.indigo[500],
        color: theme.color.white,
    },
})(MUIButton)

function Button({ primary, ...props }) {
    if (primary) {
        return <PrimaryButton variant="contained" {...props} />
    }
    return <MUIButton variant="contained" {...props}  />
}

export default Button