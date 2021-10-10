export const typography = {
    type: {
        primary: '"CallingCode", san-serif',
        code: '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace',
    },
    weight: {
        regular: '400',
        bold: '700',
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

const main = {
    name: 'Main',
    color: {
        primary: {
            main: 'red',
        },
    },
    typography,
}

const themes = [main]

export default themes
