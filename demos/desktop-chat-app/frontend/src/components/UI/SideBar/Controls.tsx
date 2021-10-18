import { useState } from 'react'
import styled from 'styled-components'

import SpeedDial from '@material-ui/lab/SpeedDial'
import SpeedDialIcon from '@material-ui/lab/SpeedDialIcon'
import SpeedDialAction from '@material-ui/lab/SpeedDialAction'
import DMIcon from '@material-ui/icons/Forum'
import AddServerIcon from '@material-ui/icons/AddCircle'
import ImportServerIcon from '@material-ui/icons/CloudDownload'
import ContactsIcon from '@material-ui/icons/PermContactCalendar'
import AppsIcon from '@material-ui/icons/Apps'
import CloseIcon from '@material-ui/icons/Close'

interface ControlsProps {
    setServerName: (serverName: string) => unknown
}

const SControlMenu = styled(SpeedDial)`
    &&& {
        .MuiSpeedDial-fab {
            height: 48px;
            width: 48px;
            border-radius: 8px;
            color: ${({ theme }) => theme.color.icon};
            background: ${({ theme }) => theme.color.iconBg};
            .MuiTouchRipple-child {
                background-color: ${({ theme }) => theme.color.ripple.dark};
            }
            &:hover {
                background: ${({ theme }) => theme.color.primary};
                transform: translate3d(0px, -2px, 0px) !important;
                box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px !important;
                color: ${({ theme }) => theme.color.iconBg};
                > svg {
                    color: ${({ theme }) => theme.color.iconBg};
                }
            }
            &:active {
                background: ${({ theme }) => theme.color.button.primaryHover};
                box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset !important;
                transform: translate3d(0px, 0px, 0px) !important;
                color: ${({ theme }) => theme.color.iconBg};
                > svg {
                    color: ${({ theme }) => theme.color.iconBg};
                }
            }
        }
        .MuiSpeedDial-actions {
            margin-bottom: -40px;
        }
    }
`

const SControlItem = styled(SpeedDialAction)`
    &&& {
        border-radius: 8px;
        height: 40px;
        width: 40px;
        .MuiSvgIcon-root {
            font-size: 24px;
        }
        color: ${({ theme }) => theme.color.icon};
        background: ${({ theme }) => theme.color.iconBg};
        .MuiTouchRipple-child {
            background-color: ${({ theme }) => theme.color.ripple.dark};
        }
        &:hover {
            background: ${({ theme }) => theme.color.primary};
            transform: translate3d(0px, -2px, 0px) !important;
            box-shadow: rgb(0 0 0 / 20%) 0px 2px 6px 0px !important;
            color: ${({ theme }) => theme.color.iconBg};
            > svg {
                color: ${({ theme }) => theme.color.iconBg};
            }
        }
        &:active {
            background: ${({ theme }) => theme.color.button.primaryHover};
            box-shadow: rgb(0 0 0 / 10%) 0px 0px 0px 3em inset !important;
            transform: translate3d(0px, 0px, 0px) !important;
            color: ${({ theme }) => theme.color.iconBg};
            > svg {
                color: ${({ theme }) => theme.color.iconBg};
            }
        }
    }
`

function Controls({ setServerName = () => false }: ControlsProps): JSX.Element {
    const [isOpen, setIsOpen] = useState(false)

    return (
        <SControlMenu
            ariaLabel="Controls Menu"
            onOpen={() => setIsOpen(true)}
            onClose={() => setIsOpen(false)}
            open={isOpen}
            icon={
                <SpeedDialIcon icon={<AppsIcon />} openIcon={<CloseIcon />} />
            }
        >
            <SControlItem
                icon={<DMIcon />}
                tooltipTitle="Direct Messages"
                onClick={() => {
                    setServerName('')
                    setIsOpen(false)
                }}
                arrow
            />
            <SControlItem
                icon={<AddServerIcon />}
                tooltipTitle="Add Server"
                onClick={() => setIsOpen(false)}
                arrow
            />
            <SControlItem
                icon={<ImportServerIcon />}
                tooltipTitle="Import Server"
                onClick={() => setIsOpen(false)}
                arrow
            />
            <SControlItem
                icon={<ContactsIcon />}
                tooltipTitle="Contacts"
                onClick={() => setIsOpen(false)}
                arrow
            />
        </SControlMenu>
    )
}

export default Controls
