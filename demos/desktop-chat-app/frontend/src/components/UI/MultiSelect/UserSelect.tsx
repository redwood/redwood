import styled from 'styled-components'
import UserAvatar from '../UserAvatar'

interface UserSelectProps {
    username?: string
    address: string
}

const SUserSelectWrapper = styled.div`
    display: flex;
    align-items: center;
    width: 100%;
    span {
        color: white;
        &:nth-child(2) {
            font-family: ${({ theme }) => theme.font.type.secondary};
            font-weight: bold;
            padding-left: 8px;
        }
        &:last-child {
            padding-left: 18px;
            font-size: 12px;
        }
    }
`

function UserSelect({
    username = '',
    address = '',
}: UserSelectProps): JSX.Element {
    return (
        <SUserSelectWrapper>
            <UserAvatar address={address} />
            <span>{username}</span>
            <span>{address}</span>
        </SUserSelectWrapper>
    )
}

export default UserSelect
