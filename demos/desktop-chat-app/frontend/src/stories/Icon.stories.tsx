import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import { Settings, ExitToApp, Face } from '@material-ui/icons'
import IconWrapper from '../components/UI/Icon/Wrapper'

import { color } from '../theme/ui'

export default {
    title: 'Icon',
    component: IconWrapper,
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

export const DefaultIcon: Story<ComponentProps<typeof IconWrapper>> = (
    args,
) => (
    <div>
        <IconWrapper {...args} size={args.size} icon={<Settings />} />
        <IconWrapper {...args} size={args.size} icon={<ExitToApp />} />
        <IconWrapper {...args} size={args.size} icon={<Face />} />
    </div>
)
DefaultIcon.args = {
    style: { padding: 8 },
}

export const LargeIcon: Story<ComponentProps<typeof IconWrapper>> = (args) => (
    <div>
        <IconWrapper {...args} icon={<Settings />} />
        <IconWrapper {...args} icon={<ExitToApp />} />
        <IconWrapper {...args} icon={<Face />} />
    </div>
)
LargeIcon.args = {
    size: 40,
    style: { padding: 8 },
}
