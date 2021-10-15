import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import { useState } from '@storybook/addons'

import UserAvatar from '../components/UI/UserAvatar'
import Header from '../components/UI/Text/Header'
import { color } from '../theme/ui'

export default {
    title: 'UserAvatar',
    component: UserAvatar,
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

const Template: Story<ComponentProps<typeof UserAvatar>> = (args) => (
    <UserAvatar {...args} />
)
Template.args = {}

export const Default = Template.bind({})
Default.args = {
    address: '2',
    large: false,
}

function GroupAvatars(args: Record<string, unknown>): JSX.Element {
    return (
        <div style={{ display: 'flex', flexDirection: 'row' }}>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="1" />
                <Template {...args} address="2" />
                <Template {...args} address="3" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="4" />
                <Template {...args} address="5" />
                <Template {...args} address="6" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="7" />
                <Template {...args} address="8" />
                <Template {...args} address="9" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="10" />
                <Template {...args} address="11" />
                <Template {...args} address="12" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="13" />
                <Template {...args} address="14" />
                <Template {...args} address="15" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="16" />
                <Template {...args} address="17" />
                <Template {...args} address="18" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="19" />
                <Template {...args} address="20" />
                <Template {...args} address="21" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="22" />
                <Template {...args} address="23" />
                <Template {...args} address="24" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="25" />
                <Template {...args} address="26" />
                <Template {...args} address="27" />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column' }}>
                <Template {...args} address="28" />
                <Template {...args} address="29" />
                <Template {...args} address="30" />
            </div>
        </div>
    )
}

export const GroupDefault: Story<ComponentProps<typeof UserAvatar>> = (
    args,
) => (
    <div>
        <Header style={{ paddingLeft: 16 }} isSmall>
            Randomly Generated User Avatars
        </Header>
        <GroupAvatars {...args} />
    </div>
)

export const GroupCircle = GroupDefault.bind({})
GroupCircle.args = { circle: true }

export const Large = Default.bind({})
Large.args = { large: true }

export const Image: Story<ComponentProps<typeof UserAvatar>> = (args) => (
    <div style={{ display: 'flex' }}>
        <UserAvatar
            {...args}
            imageSrc="https://www.placecage.com/c/300/300"
            address="1"
        />
        <UserAvatar
            {...args}
            imageSrc="https://www.placecage.com/c/300/301"
            address="2"
        />
        <UserAvatar
            {...args}
            imageSrc="https://www.placecage.com/c/300/302"
            address="3"
        />
        <UserAvatar
            {...args}
            imageSrc="https://www.placecage.com/c/300/303"
            address="4"
        />
        <UserAvatar
            {...args}
            imageSrc="https://www.placecage.com/c/300/304"
            address="5"
        />
    </div>
)

export const CircleImage = Image.bind({})
CircleImage.args = { circle: true }

export const LargeImage = Image.bind({})
LargeImage.args = { large: true }

export const LoadingImage = Image.bind({})
LoadingImage.args = { loadingImage: true }

export const LargeLoadingImage = Image.bind({})
LargeLoadingImage.args = { large: true, loadingImage: true }

function retryTimeout(
    setLoading: (val: boolean) => void,
    setFailed: (val: boolean) => void,
) {
    setLoading(true)
    setTimeout(() => {
        setLoading(false)
        setFailed(false)
    }, 2000)
}
export const FailedImage: Story<ComponentProps<typeof UserAvatar>> = (args) => {
    const [isLoading1, setIsLoading1] = useState(false)
    const [isLoading2, setIsLoading2] = useState(false)
    const [isLoading3, setIsLoading3] = useState(false)
    const [isLoading4, setIsLoading4] = useState(false)
    const [isLoading5, setIsLoading5] = useState(false)

    const [loadingFailed1, setLoadingFailed1] = useState(true)
    const [loadingFailed2, setLoadingFailed2] = useState(true)
    const [loadingFailed3, setLoadingFailed3] = useState(true)
    const [loadingFailed4, setLoadingFailed4] = useState(true)
    const [loadingFailed5, setLoadingFailed5] = useState(true)
    return (
        <div style={{ display: 'flex' }}>
            <UserAvatar
                {...args}
                onRetry={() => retryTimeout(setIsLoading1, setLoadingFailed1)}
                loadingImage={isLoading1}
                loadFailed={loadingFailed1}
                imageSrc="https://www.placecage.com/c/300/300"
                address="1awmdklawdk"
            />
            <UserAvatar
                {...args}
                imageSrc="https://www.placecage.com/c/300/301"
                onRetry={() => retryTimeout(setIsLoading2, setLoadingFailed2)}
                loadingImage={isLoading2}
                loadFailed={loadingFailed2}
                address="2awdjnawd"
            />
            <UserAvatar
                {...args}
                imageSrc="https://www.placecage.com/c/300/302"
                onRetry={() => retryTimeout(setIsLoading3, setLoadingFailed3)}
                loadingImage={isLoading3}
                loadFailed={loadingFailed3}
                address="dwdalakd3"
            />
            <UserAvatar
                {...args}
                imageSrc="https://www.placecage.com/c/300/303"
                onRetry={() => retryTimeout(setIsLoading4, setLoadingFailed4)}
                loadingImage={isLoading4}
                loadFailed={loadingFailed4}
                address="4fnwffk"
            />
            <UserAvatar
                {...args}
                imageSrc="https://www.placecage.com/c/300/304"
                onRetry={() => retryTimeout(setIsLoading5, setLoadingFailed5)}
                loadingImage={isLoading5}
                loadFailed={loadingFailed5}
                address="5fmeklw"
            />
        </div>
    )
}
