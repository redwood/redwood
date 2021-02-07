import React, { useState } from 'react'
import styled from 'styled-components'
import { makeStyles } from '@material-ui/core/styles'
import { Button as MUIButton } from '@material-ui/core'
import theme from '../../theme'

const usePrimaryButtonStyles = makeStyles({
    root: {
        backgroundColor: theme.color.indigo[500],
        color: theme.color.white,
    },
    disabled: {
        '&&': {
            backgroundColor: theme.color.grey[100],
        },
        '&&:hover': {
            cursor: 'pointer',
        },
    },
})

const useNonPrimaryButtonStyles = makeStyles({
    root: {
        backgroundColor: theme.color.grey[200],
        color: theme.color.white,
    },
    disabled: {
        '&&': {
            backgroundColor: theme.color.grey[100],
            cursor: 'not-allowed',
        },
    },
})

function Button({ primary, ...props }) {
    const primaryButtonStyles = usePrimaryButtonStyles()
    const nonPrimaryButtonStyles = useNonPrimaryButtonStyles()
    if (primary) {
        return <MUIButton variant="contained" {...props} classes={primaryButtonStyles} />
    }
    return <MUIButton variant="contained" {...props} classes={nonPrimaryButtonStyles} />
}

export default Button