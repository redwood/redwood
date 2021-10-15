import { ComponentProps } from 'react'
import { Story } from '@storybook/react'

import MultiSelect from '../components/UI/MultiSelect'
import { color } from '../theme/ui'

export default {
    title: 'MultiSelect',
    component: MultiSelect,
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

const Template: Story<ComponentProps<typeof MultiSelect>> = (args) => (
    <MultiSelect {...args} />
)
Template.args = {}

export const Basic = Template.bind({})
Basic.args = {}
