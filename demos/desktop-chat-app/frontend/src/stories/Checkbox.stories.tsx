import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import { useState } from '@storybook/addons'

import CheckBox from '../components/UI/CheckBox'
import { color } from '../theme/ui'

export default {
    title: 'CheckBox',
    component: CheckBox,
    parameters: {
        backgrounds: {
            default: 'dark',
            values: [
                { name: 'dark', value: color.background },
                { name: 'light', value: color.background },
            ],
        },
    },
}

const Template: Story<ComponentProps<typeof CheckBox>> = (args) => {
    const [checked, setChecked] = useState(false)
    return (
        <CheckBox
            {...args}
            checked={checked}
            onValueChanged={(value) => setChecked(value)}
        />
    )
}
Template.args = {
    disabled: false,
}

export const Primary = Template.bind({})
Primary.args = {
    ...Template.args,
    sType: 'primary',
}

export const Secondary = Template.bind({})
Secondary.args = {
    ...Template.args,
    sType: 'secondary',
}

export const Disabled = Template.bind({})
Disabled.args = {
    ...Template.args,
    sType: 'primary',
    disabled: true,
}

export const Label = Template.bind({})
Label.args = {
    ...Template.args,
    sType: 'primary',
    label: 'Add Peers',
}

export const LabelRight = Template.bind({})
LabelRight.args = {
    ...Template.args,
    sType: 'primary',
    label: 'Add Peers',
    labelPlacement: 'end',
}
