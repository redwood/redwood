import { FunctionComponent, ComponentProps } from 'react'
import Header from '../components/UI/Text/Header'

import { color } from '../theme/ui'

export default {
    title: 'text/Header',
    component: Header,
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

export const Default: FunctionComponent<ComponentProps<typeof Header>> = (
    args,
) => <Header {...args}>Header Text Here</Header>

export const SubHeader: FunctionComponent<ComponentProps<typeof Header>> = (
    args,
) => (
    <Header {...args} isSmall>
        SubHeader Text Here
    </Header>
)
