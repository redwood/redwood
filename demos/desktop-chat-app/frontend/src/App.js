import React, { useState, useRef } from 'react'
import {
  BrowserRouter as Router,
  Switch,
  Route,
} from "react-router-dom";

import Main from './Main'
import SignIn from './components/Account/SignIn'
import SignUp from './components/Account/SignUp'
import Profiles from './components/Account/Profiles'

function App() {
    return (
      <Router>
        <div>
          <Switch>
            <Route exact path="/">
              <Main />
            </Route>
            <Route path="/signin">
              <SignIn />
            </Route>
            <Route path="/signup">
              <SignUp />
            </Route>
            <Route path="/profiles">
              <Profiles />
            </Route>
          </Switch>
        </div>
      </Router>
    )
}

export default App
