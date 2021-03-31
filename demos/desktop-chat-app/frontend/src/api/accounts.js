export default function(redwood, history) {
  async function logout(redirect) {
    const logoutResp = await fetch('http://localhost:54231/api/logout', { method: 'POST' })
    console.log(logoutResp)
    console.log('LOGGED OUT')
    if (logoutResp.status === 500) {
      console.log('Error logging out')
      return
    }
    if (redirect) {
      history.push('/signin')
    }
  }

  async function checkLogin() {
    try {
    let resp = await fetch('http://localhost:54231/api/check-login', { method: 'POST' })
  
      const jsonResp = await resp.text()
  
      return jsonResp === 'true'
    } catch (err) {
      console.error(err)
    }
  }
  
  async function onSignIn(setErrorMessage, setLoadingText, data) {
    try {
      setErrorMessage('')
      setLoadingText('Signing into profile...')
      let resp = await fetch('http://localhost:54231/api/login', {
        method: 'POST',
        body: JSON.stringify({
          profileName: data.profileName,
          mnemonic: data.mnemonic,
          password: data.password,
        }),
      })
  
      if (resp.status === 500) {
        const errorText = await resp.text()
        setErrorMessage(errorText)
        setLoadingText('')
        return
      }
  
      let redwoodClient = redwood.fetchRedwoodClient()
      await redwood.fetchIdentities(redwoodClient)
      setLoadingText('')
      history.push('/')
    } catch (err) {
      console.error(err)
      setLoadingText('')
    }
  }

  async function confirmProfile(setErrorMessage, setLoadingText, setMnemonic, data) {
    try {
      setErrorMessage('')
      setLoadingText('Creating profile...')

      if (data.password !== data.confirmPassword) {
        setErrorMessage('Passwords do not match.')
        setLoadingText('')
        return
      }

      let resp = await fetch('http://localhost:54231/api/confirm-profile', {
        method: 'POST',
        // headers: {
        //   'Content-Type': 'application/json',
        // },
        body: JSON.stringify({
          profileName: data.profileName,
        }),
      })

      if (resp.status === 500) {
        const errorText = await resp.text()
        setLoadingText('')
        setErrorMessage(errorText)
        return
      }
      const createdMnemonic = await resp.text()
      setMnemonic(createdMnemonic)
      setLoadingText('')
    } catch (err) {
      setLoadingText('')
      setErrorMessage('')
      console.error(err)
    }
  }

  async function onSignUp(setErrorMessage, setLoadingText, data) {
    try {
      setErrorMessage('')
      setLoadingText('Validating and generating mnemonic...')
      let resp = await fetch('http://localhost:54231/api/login', {
        method: 'POST',
        // headers: {
        //   'Content-Type': 'application/json',
        // },
        body: JSON.stringify({
          profileName: data.profileName,
          mnemonic: data.mnemonic,
          password: data.password,
        }),
      })

      if (resp.status === 500) {
        const errorText = await resp.text()
        setLoadingText('')
        setErrorMessage(errorText)
        return
      }


      let redwoodClient = redwood.fetchRedwoodClient()
      await redwood.fetchIdentities(redwoodClient)
      setLoadingText('')
      history.push('/')
    } catch (err) {
      setLoadingText('')
      setErrorMessage('')
      console.error(err)
    }
  }

  async function getProfileNames() {
    try {
      let resp = await (await fetch('http://localhost:54231/api/profile-names', { method: 'GET' })).json()

      return resp.profileNames || []
    } catch (err) {
      return err
    }
  }

  async function signInProfile(setErrorMessage, setLoadingText, data) {
    try {
      setErrorMessage('')
      setLoadingText('Signing into profile...')
      const isLoggedIn = await checkLogin()

      if (isLoggedIn) {
        await logout()
      }

      let resp = await fetch('http://localhost:54231/api/login', {
        method: 'POST',
        body: JSON.stringify({
          profileName: data.selectedProfile,
          password: data.password,
        }),
      })

      if (resp.status === 500) {
        const errorText = await resp.text()
        setErrorMessage(errorText)
        setLoadingText('')
        return
      }
      
      let redwoodClient = redwood.fetchRedwoodClient()
      await redwood.fetchIdentities(redwoodClient)
      history.push('/')
    } catch (err) {
      setErrorMessage('')
      setLoadingText('')
      console.error(err)
    }
  }

  return {
    checkLogin,
    onSignIn,
    confirmProfile,
    onSignUp,
    signInProfile,
    getProfileNames,
    logout,
  }
}