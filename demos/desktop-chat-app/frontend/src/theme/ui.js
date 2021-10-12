export const font = {
    type: {
        primary: '"CallingCode", san-serif',
        secondary: '"Rogan", san-serif',
        code: '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace',
    },
    weight: {
        regular: '400',
        medium: '500',
        semiBold: '600',
        bold: '700',
        extra: '900',
    },
    size: {
        s1: 12,
        s2: 14,
        s3: 16,
        m1: 20,
        m2: 24,
        m3: 28,
        l1: 32,
        l2: 40,
        l3: 48,
        code: 90,
    },
}

export const color = {
    primary: '#76F595',
    button: {
        primaryHover: '#2ecd55',
        primaryActive: '#16af3b',
        disabledBg: '#53525B',
        disabledColor: '#B4B3C3',
    },
    secondary: '#E84652',
    background: '#0E0C2C',
    elevation1: '#19154B',
    elevation2: '#211D61',
    icon: '#C4C3D8',
    iconBg: '#292832',
    accent1: '#53525B',
    accent2: '#B4B3C3',
    text: 'rgb(255, 255, 255)',
    textDark: '#15141B',
    ripple: {
        primary: '#FFFFFF',
        secondary: '#303F9F',
        dark: '#000000',
    },
}

export const icon = {
    size: 20,
}

export const transition = {
    primary: 'all ease-in-out .15s',
    cubicBezier: 'transform 700ms cubic-bezier(0.175, 0.885, 0.335, 1.05) 0s',
}

const main = {
    name: 'Main',
    color,
    font,
    icon,
    transition,
}

const themes = [main]

export default themes
