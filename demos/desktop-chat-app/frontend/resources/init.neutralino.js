/* eslint-disable */
window.myApp = {
    onWindowClose: () => {
        Neutralino.app.exit()
    },
}

Neutralino.init()
Neutralino.events.on('windowClose', myApp.onWindowClose)
//window.myApp.setTray(); Uncomment to enable the tray menu.
window.myApp.showInfo()
/* eslint-disable */
