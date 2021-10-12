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
    type: 'text',
    id: 'username',
    placeholder: 'Enter a username...',
}

export const Basic = Template.bind({})
Basic.args = { ...Template.args }

export const Label = Template.bind({})
Label.args = {
    ...Template.args,
    label: 'Username',
}

export const Disabled = Template.bind({})
Disabled.args = {
    ...Template.args,
    disabled: true,
}

export const Error = Template.bind({})
Error.args = {
    ...Template.args,
    label: 'Username',
    errorText: 'Username is required.',
}
