import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import { useState } from '@storybook/addons'

import SideBar from '../components/UI/SideBar'
import ServerButton from '../components/UI/SideBar/ServerButton'
import { color } from '../theme/ui'

export default {
    title: 'SideBar',
    component: SideBar,
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

export const Basic: Story<ComponentProps<typeof SideBar>> = (args) => {
    const [selectedServer, setSelectedServer] = useState('')

    return (
        <SideBar setServerName={setSelectedServer} {...args}>
            <>
                <ServerButton
                    serverName="default.chat"
                    selected={selectedServer === 'default.chat'}
                    onClick={() => setSelectedServer('default.chat')}
                />
                <ServerButton
                    serverName="general.chat"
                    selected={selectedServer === 'general.chat'}
                    onClick={() => setSelectedServer('general.chat')}
                />
                <ServerButton
                    serverName="funny.memes"
                    selected={selectedServer === 'funny.memes'}
                    onClick={() => setSelectedServer('funny.memes')}
                />
            </>
        </SideBar>
    )
}
Basic.args = {
    controlStyle: 'fab',
}

export const Squared = Basic.bind({})
Squared.args = {
    controlStyle: 'squared',
}
