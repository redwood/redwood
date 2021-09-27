import React from 'react'
import clsx from 'clsx'
import { makeStyles, withStyles } from '@material-ui/core/styles'
import InputLabel from '@material-ui/core/InputLabel'
import MenuItem from '@material-ui/core/MenuItem'
import FormControl from '@material-ui/core/FormControl'
import Select from '@material-ui/core/Select'
import InputBase from '@material-ui/core/InputBase'
import theme from '../../theme'

const BootstrapInput = withStyles((muiTheme) => ({
    root: {
        label: {
            paddingTop: 10,
        },

        'label + &': {
            marginTop: 20, // theme.spacing(3),
        },
    },
    input: {
        borderRadius: 4,
        position: 'relative',
        backgroundColor: theme.color.grey[300],
        border: '1px solid #ced4da',
        fontSize: 16,
        padding: '10px 26px 10px 12px',
        transition: muiTheme.transitions.create(['border-color', 'box-shadow']),
        // NOTE: Use the system font instead of the default Roboto font.
        fontFamily: [
            '-apple-system',
            'BlinkMacSystemFont',
            '"Segoe UI"',
            'Roboto',
            '"Helvetica Neue"',
            'Arial',
            'sans-serif',
            '"Apple Color Emoji"',
            '"Segoe UI Emoji"',
            '"Segoe UI Symbol"',
        ].join(','),

        '&:focus': {
            borderRadius: 4,
            borderColor: '#80bdff',
            boxShadow: '0 0 0 0.2rem rgba(0,123,255,.25)',
        },
    },
}))(InputBase)

const useFormControlStyles = makeStyles(() => ({
    margin: {
        width: '100%',
    },
}))

const useLabelStyles = makeStyles({
    root: {
        color: theme.color.white,
    },
    focused: {
        '&&': {
            color: theme.color.white,
        },
    },
})

const useSelectStyles = makeStyles({
    root: {
        color: theme.color.white,
        borderColor: theme.color.grey[100],
        borderRadius: 12,
    },
    select: {
        '&&': {
            backgroundColor: theme.color.grey[100],
        },
    },
    selectMenu: {
        backgroundColor: theme.color.grey[100],
    },
})

const useSelectMenuListStyles = makeStyles({
    root: {
        color: theme.color.white,
        backgroundColor: theme.color.grey[100],
    },
})

const useSelectMenuStyles = makeStyles({
    paper: {
        backgroundColor: theme.color.grey[100],
    },
})

const useSelectMenuItemStyles = makeStyles({
    root: {
        '&&:hover': {
            backgroundColor: theme.color.grey[200],
        },
    },
    selected: {
        '&&': {
            backgroundColor: theme.color.grey[300],
        },
        '&&:hover': {
            backgroundColor: theme.color.grey[200],
        },
    },
})

function CustomSelect({ label, items, value, onChange, className }) {
    const formControlClasses = useFormControlStyles()
    const labelClasses = useLabelStyles()
    const selectClasses = useSelectStyles()
    const selectMenuListClasses = useSelectMenuListStyles()
    const selectMenuClasses = useSelectMenuStyles()
    const selectMenuItemClasses = useSelectMenuItemStyles()
    return (
        <FormControl className={clsx(formControlClasses.margin, className)}>
            <InputLabel
                shrink
                id="demo-customized-select-label"
                classes={labelClasses}
            >
                {label}
            </InputLabel>
            <Select
                labelId="demo-customized-select-label"
                id="demo-customized-select"
                classes={selectClasses}
                value={value}
                onChange={onChange}
                input={<BootstrapInput />}
                MenuProps={{
                    MenuListProps: { classes: selectMenuListClasses },
                    classes: selectMenuClasses,
                }}
            >
                {items.map((item) => (
                    <MenuItem
                        key={item.value}
                        value={item.value}
                        classes={selectMenuItemClasses}
                    >
                        {item.text}
                    </MenuItem>
                ))}
            </Select>
        </FormControl>
    )
}

export default CustomSelect
