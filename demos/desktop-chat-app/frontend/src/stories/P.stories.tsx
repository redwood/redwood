import { ComponentProps } from 'react'
import { Story, Meta } from '@storybook/react'
import P from '../components/UI/Text/P'

import { color } from '../theme/ui'

export default {
    title: 'text/Paragraph',
    component: P,
    parameters: {
        backgrounds: {
            default: 'dark',
            values: [
                { name: 'dark', value: color.background },
                { name: 'light', value: color.background },
            ],
        },
    },
} as Meta

export const Template: Story<ComponentProps<typeof P>> = (args) => (
    <P {...args}>Paragraph text here...</P>
)
Template.args = {}

export const SubParagraph = Template.bind({})
SubParagraph.args = { isSmall: true }
