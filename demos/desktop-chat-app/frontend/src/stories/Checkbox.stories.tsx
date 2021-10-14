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
    const [isChecked, setIsChecked] = useState(false)
    return (
        <CheckBox
            {...args}
            isChecked={isChecked}
            onValueChanged={(value) => setIsChecked(value)}
        />
    )
}
Template.args = {}

export const Primary = Template.bind({})
Primary.args = {
    sType: 'primary',
}

export const Secondary = Template.bind({})
Secondary.args = {
    sType: 'secondary',
}

export const Disabled = Template.bind({})
Disabled.args = {
    sType: 'primary',
    disabled: true,
}

export const Label = Template.bind({})
Label.args = {
    sType: 'primary',
    label: 'Add Peers',
}

export const LabelRight = Template.bind({})
LabelRight.args = {
    sType: 'primary',
    label: 'Add Peers',
    labelPlacement: 'end',
}
