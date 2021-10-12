import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import Face from '@material-ui/icons/Face'

import Button from '../components/UI/Button'
import { color } from '../theme/ui'

export default {
    title: 'Button',
    component: Button,
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

const Template: Story<ComponentProps<typeof Button>> = (args) => (
    <Button {...args} />
)
Template.args = {
    onClick: () => 'clicked',
}

export const Basic = Template.bind({})
Basic.args = {
    label: 'Label',
    sType: 'primary',
}

export const Outline = Template.bind({})
Outline.args = {
    label: 'Outline',
    sType: 'outline',
}

export const Disabled = Template.bind({})
Disabled.args = {
    label: 'Disabled',
    sType: 'primary',
    disabled: true,
}

export const Icon = Template.bind({})
Icon.args = {
    sType: 'primary',
    icon: <Face />,
}
