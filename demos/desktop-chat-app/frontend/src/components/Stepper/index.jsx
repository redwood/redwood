import React from 'react'
import styled from 'styled-components'
import clsx from 'clsx'
import { Stepper, Step, StepLabel, StepConnector } from '@material-ui/core'
import { Check as CheckIcon } from '@material-ui/icons'
import { makeStyles } from '@material-ui/core/styles'
import theme from '../../theme'

function CustomStepper({ steps, activeStep }) {
    return (
        <SStepper activeStep={0} connector={<SStepConnector />}>
            {steps.map((label, i) => (
                <Step
                    key={label}
                    active={i === activeStep}
                    completed={i < activeStep}
                >
                    <SStepLabel StepIconComponent={StepIcon}>
                        {label}
                    </SStepLabel>
                </Step>
            ))}
        </SStepper>
    )
}

export default CustomStepper

const SStepper = styled((props) => (
    <Stepper {...props} classes={{ root: 'root' }} />
))`
    && {
        background-color: unset;
        padding: 0 24px 48px;
    }
    && .root {
        color: ${(props) => props.theme.color.white};
    }
    && .label {
        color: ${(props) => props.theme.color.white};
    }
    && .completed {
        color: ${(props) => props.theme.color.white};
    }
`

const SStepLabel = styled((props) => (
    <StepLabel
        {...props}
        classes={{ label: 'label', active: 'active', completed: 'completed' }}
    />
))`
    && .active {
        color: ${(props) => props.theme.color.white};
    }
    && .label {
        color: ${(props) => props.theme.color.white};
    }
    && .completed {
        color: ${(props) => props.theme.color.white};
    }
`

const SStepConnector = styled((props) => (
    <StepConnector {...props} classes={{ line: 'line' }} />
))`
    && .line {
        border-top-style: dashed;
        border-color: ${(props) => props.theme.color.grey[100]};
    }
`

const useStepIconStyles = makeStyles({
    root: {
        color: theme.color.white,
        display: 'flex',
        height: 22,
        alignItems: 'center',
        '& svg': {
            width: 22,
            height: 22,
        },
    },
    active: {
        '& $circle': {
            backgroundColor: theme.color.indigo[500],
        },
    },
    circle: {
        width: 20,
        height: 20,
        borderRadius: '50%',
        fontSize: '0.8rem',
        lineHeight: '0.8rem',
        backgroundColor: theme.color.grey[200],
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
    },
    completed: {
        color: theme.color.indigo[500],
        zIndex: 1,
        fontSize: 18,
        '& $circle': {
            backgroundColor: 'transparent', // theme.color.indigo[500],
        },
    },
})

function StepIcon({ active, completed, icon }) {
    const classes = useStepIconStyles()

    return (
        <div
            className={clsx(classes.root, {
                [classes.active]: active,
                [classes.completed]: completed,
            })}
        >
            <div className={classes.circle}>
                {completed ? <CheckIcon /> : <div>{icon}</div>}
            </div>
        </div>
    )
}
