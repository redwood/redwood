import { ComponentProps } from 'react'
import { Story } from '@storybook/react'

import Button from '../components/UI/Button'
import { color } from '../theme/ui'

export default {
    title: 'Template',
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
