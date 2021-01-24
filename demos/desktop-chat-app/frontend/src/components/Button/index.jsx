import React, { useState } from 'react'
import styled from 'styled-components'
import { withStyles } from '@material-ui/core/styles'
import { Button as MUIButton } from '@material-ui/core'
import theme from '../../theme'

const PrimaryButton = withStyles({
    root: {
        backgroundColor: theme.color.green[500],
    },
})(MUIButton)

function Button({ primary, ...props }) {
        console.log(primary)
    if (primary) {
        return <PrimaryButton variant="contained" {...props} />
    }
    return <MUIButton variant="contained" {...props}  />
}

export default Button