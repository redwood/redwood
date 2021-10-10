interface HeaderProps {
    text: string
}

function Header({ text }: HeaderProps): JSX.Element {
    return <h2>{text}</h2>
}

export default Header
