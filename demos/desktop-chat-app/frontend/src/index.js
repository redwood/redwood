import './wdyr'
import ReactDOM from 'react-dom'

import App from './App'
import reportWebVitals from './reportWebVitals'

import './index.css'

ReactDOM.render(<App />, document.getElementById('root'))

window.Neutralino.init()

setTimeout(async () => {
    console.log(
        await window.Neutralino.os.execCommand({
            command: `echo "hello world"`,
        }),
    )
})

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals

reportWebVitals()
