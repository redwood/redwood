import { CSSProperties } from 'react'
import styled, { css } from 'styled-components'
import DMIcon from '@material-ui/icons/Forum'
import AddServerIcon from '@material-ui/icons/AddCircle'
import ImportServerIcon from '@material-ui/icons/CloudDownload'
import ContactsIcon from '@material-ui/icons/PermContactCalendar'

import Controls from './Controls'

import Button from '../Button'

interface SideBarProps {
    style?: CSSProperties
    setServerName?: (serverName: string) => unknown
    children: JSX.Element
    controlStyle?: 'squared' | 'fab'
}

const SSideBar = styled.div`
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: space-between;
    min-height: 600px; // Remove this later
    width: 64px;
    background: ${({ theme }) => theme.color.textDark};
    box-shadow: rgb(0 0 0 / 20%) 0px 2px 1px -1px,
        rgb(0 0 0 / 14%) 0px 1px 1px 0px, rgb(0 0 0 / 12%) 0px 1px 3px 0px;
`

const SSideBarTop = styled.div`
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding-top: 8px;
    .MuiButtonBase-root {
        margin-bottom: 12px;
    }
`

const SControlBtn = styled(Button)`
    &&& {
        width: 24px;
        height: 24px;

        .MuiSvgIcon-root {
            font-size: 12px;
        }
    }
`

const SControlRow = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
`

const SSideBarBottom = styled.div`
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding-bottom: 8px;
`

function SideBar({
    style = {},
    children,
    controlStyle = 'squared',
    setServerName = () => false,
}: SideBarProps): JSX.Element {
    let controls = (
        <>
            <SControlRow style={{ marginBottom: 4 }}>
                <SControlBtn
                    style={{ marginRight: 4 }}
                    sType="primary"
                    onClick={() => false}
                    icon={<DMIcon />}
                />
                <SControlBtn
                    sType="primary"
                    onClick={() => false}
                    icon={<ContactsIcon />}
                />
            </SControlRow>
            <SControlRow>
                <SControlBtn
                    style={{ marginRight: 4 }}
                    sType="primary"
                    onClick={() => false}
                    icon={<ImportServerIcon />}
                />
                <SControlBtn
                    sType="primary"
                    onClick={() => false}
                    icon={<AddServerIcon />}
                />
            </SControlRow>
        </>
    )

    if (controlStyle === 'fab') {
        controls = <Controls setServerName={setServerName} />
    }

    return (
        <SSideBar style={style}>
            <SSideBarTop>{children}</SSideBarTop>
            <SSideBarBottom>{controls}</SSideBarBottom>
        </SSideBar>
    )
}

export default SideBar
