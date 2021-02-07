
export const white = 'rgba(255,255,255,0.8)'
export const black = '#000'

export const green = {
    500: '#42a064',
}

export const indigo = {
  500: 'hsl(231deg 36% 53%)',
}

export const red = {
    100: '#FFFDFE',
    200: '#ffc2a8',
    500: '#d16c00',
}

export const grey = {
    600: 'hsl(220 8% 12% / 1)',
    500: 'hsl(220 8% 16% / 1)',
    400: 'hsl(220 8% 18% / 1)',
    300: 'hsl(220 8% 21% / 1)',
    200: 'hsl(220 8% 24% / 1)',
    100: 'hsl(220 8% 36% / 1)',
}

const theme = {
    borderRadius: 12,
    breakpoints: {
        mobile: 400,
    },
    color: {
        primary: {
            light: black,
            main: black,
        },
        secondary: {
            main: indigo[500],
        },
        black,
        grey,
        green,
        white,
        red,
        indigo,
    },
    siteWidth: 1200,
    spacing: {
        1: 4,
        2: 8,
        3: 16,
        4: 24,
        5: 32,
        6: 48,
        7: 64,
    },
    topBarSize: 56
}

export default theme
