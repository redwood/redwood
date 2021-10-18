import { ComponentProps } from 'react'
import { Story } from '@storybook/react'

import { color } from '../theme/ui'
import ChatBar from '../components/UI/ChatBar'

export default {
    title: 'ChatBar',
    component: ChatBar,
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

const Template: Story<ComponentProps<typeof ChatBar>> = (args) => (
    <ChatBar {...args} />
)
Template.args = {}

export const Basic = Template.bind({})
Basic.args = { stateURI: 'Developers' }
