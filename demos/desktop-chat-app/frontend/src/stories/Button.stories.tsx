import { ComponentProps } from 'react'
import { Story } from '@storybook/react'

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

export const Basic: Story<ComponentProps<typeof Button>> = (args) => (
    <Button {...args} />
)
Basic.args = {
    label: 'Label',
    onClick: () => 'clicked',
    sType: 'primary',
}

export const Disabled: Story<ComponentProps<typeof Button>> = (args) => (
    <Button {...args} />
)
Disabled.args = {
    label: 'Disabled',
    onClick: () => 'clicked',
    sType: 'primary',
    disabled: true,
}
