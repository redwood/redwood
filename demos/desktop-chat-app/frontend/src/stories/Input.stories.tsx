import { ComponentProps } from 'react'
import { Story } from '@storybook/react'

import Input from '../components/UI/Input'
import { color } from '../theme/ui'

export default {
    title: 'Input',
    component: Input,
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

const Template: Story<ComponentProps<typeof Input>> = (args) => (
    <Input {...args} />
)
Template.args = {
    onChange: () => 'clicked',
    value: '',
}

export const Basic = Template.bind({})
Basic.args = {
    placeholder: 'Enter a password...',
}
