
export const white = '#FFF'
export const black = '#000'

export const green = {
  500: '##00d1810',
}

export const red = {
  100: '#FFFDFE',
  200: '#ffc2a8',
  500: '#d16c00',
}

export const grey = {
  100: '#f7f4f2',
  200: '#ececec',
  300: '#e2d6cf',
  400: '#aa9585',
  500: '#805e49',
  600: '#636363aa',
  800: '#2d1e12',
}

export const kindOfBlue = '#54549e'

const theme = {
    borderRadius: 12,
    breakpoints: {
        mobile: 400,
    },
    color: {
        black,
        grey,
        kindOfBlue,
        primary: {
            light: black,
            main: black,
        },
        secondary: {
            main: green[500],
        },
        white,
        red
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
    topBarSize: 72
}

export default theme
