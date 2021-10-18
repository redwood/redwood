import { ComponentProps } from 'react'
import { Story } from '@storybook/react'
import { useState } from '@storybook/addons'

import MultiSelect from '../components/UI/MultiSelect'
import UserSelect from '../components/UI/MultiSelect/UserSelect'
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

export const Basic: Story<ComponentProps<typeof MultiSelect>> = (args) => {
    const [select1, setSelect1] = useState(false)
    const [select2, setSelect2] = useState(false)
    const [select3, setSelect3] = useState(false)
    const [select4, setSelect4] = useState(false)
    const [select5, setSelect5] = useState(false)

    return (
        <div style={{ width: 400 }}>
            <MultiSelect
                {...args}
                checked={select1}
                onValueChanged={() => setSelect1(!select1)}
            >
                <UserSelect username="Tim Shenk" address="daml449rwdkopa0" />
            </MultiSelect>
            <MultiSelect
                {...args}
                checked={select2}
                onValueChanged={() => setSelect2(!select2)}
            >
                <UserSelect username="Chris P" address="aeml4494kekopa0" />
            </MultiSelect>
            <MultiSelect
                {...args}
                checked={select3}
                onValueChanged={() => setSelect3(!select3)}
            >
                <UserSelect username="Ezra Jackson" address="efd03ri03r3i0s" />
            </MultiSelect>
            <MultiSelect
                {...args}
                checked={select4}
                onValueChanged={() => setSelect4(!select4)}
            >
                <UserSelect username="Effren Pickles" address="dmwo30ir03ekd" />
            </MultiSelect>
            <MultiSelect
                {...args}
                checked={select5}
                onValueChanged={() => setSelect5(!select5)}
            >
                <UserSelect username="Sam Large" address="3r03riwqoekd" />
            </MultiSelect>
        </div>
    )
}
