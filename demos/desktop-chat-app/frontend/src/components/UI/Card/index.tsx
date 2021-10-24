import styled from 'styled-components'

interface CardProps {
    className?: string
    children: JSX.Element
}

const SCard = styled.div`
    background: ${({ theme }) => theme.color.iconBg};
    height: 400px;
    width: 400px;
    box-shadow: rgb(0 0 0 / 20%) 0px 2px 1px -1px,
        rgb(0 0 0 / 14%) 0px 1px 1px 0px, rgb(0 0 0 / 12%) 0px 1px 3px 0px;
    border-radius: 4px;
`

function Card({ className = '', children }: CardProps): JSX.Element {
    return <SCard className={className}>{children}</SCard>
}

export default Card
