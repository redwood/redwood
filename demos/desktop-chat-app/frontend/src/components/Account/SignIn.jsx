import React, { Fragment, useState, useEffect } from 'react'
import styled from 'styled-components'
import { Link, useHistory, Redirect } from 'react-router-dom'
import { useRedwood } from 'redwood/dist/main/react'

import Input, { InputLabel } from './../Input'
import Button from './../Button'
import Loading from './Loading'
import createAccountsApi from './../../api/accounts'

const SAccount = styled.section`
  background-color: ${props => props.theme.color.grey[500]};
  height: 100vh;
  width: 100vw;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
`

const SAccountHeader = styled.div`
  height: 250px;
  background: transparent;
  width: 100%;
`

const SAccountCard = styled.div`
  width: 450px;
  background: #36393f;
  box-shadow: 0 2px 10px 0 rgb(0 0 0 / 20%);
  border-radius: 4px;
  padding-bottom: 24px;
  position: relative;
`

const SAccountCardHeader = styled.h2`
  color: white;
  font-size: 28px;
  margin-top: 24px;
  margin-bottom: 8px;
  text-align: center;
`

const SAccountCardDesc = styled.p`
  color: rgba(255, 255, 255, .8);
  font-size: 12px;
  text-align: center;
`

const SAccountCardContent = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 16px;
`

const SLink = styled(Link)`
  font-size: 10px;
  color: #635bff;
  margin-top: 8px;
`

const SErrorMessage = styled.div`
  font-size: 10px;
  color: red;
`

function SignIn(props) {

  return (
    <Fragment>
      {props.errorMessage ? <SErrorMessage>{props.errorMessage}</SErrorMessage> : null}
      <InputLabel
        label={'Profile Name'}
      >
        <Input
          value={props.profileName}
          onChange={(event) => props.setProfileName(event.currentTarget.value)}
          type={'text'}
        />
      </InputLabel>
      <InputLabel
        label={'Mnemonic'}
      >
        <Input
          value={props.mnemonic}
          onChange={(event) => props.setMnemonic(event.currentTarget.value)}
          type={'password'}
        />
      </InputLabel>
      <InputLabel
        label={'Password'}
      >
        <Input
          value={props.password}
          onChange={(event) => props.setPassword(event.currentTarget.value)}
          type={'password'}
        />
      </InputLabel>
      <SLink to={'/profiles'}>Existing profiles.</SLink>
      <SLink to={'/signup'}>Create an account.</SLink>
      <Button
        onClick={() => props.onSignIn(
          props.setErrorMessage,
          props.setLoadingText,
          {
            profileName: props.profileName,
            mnemonic: props.mnemonic,
            password: props.password,
          }
        )}
        primary
        style={{ width: '100%', marginTop: 12 }}
        disabled={!(!!props.mnemonic && !!props.profileName && !!props.password)}
      >Sign In</Button>
    </Fragment>
  )
}

function Account(props) {
  const redwood = useRedwood()
  const history = useHistory()

  const [mnemonic, setMnemonic] = useState('')
  const [profileName, setProfileName] = useState('')
  const [password, setPassword] = useState('')
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const [loadingText, setLoadingText] = useState('')

  const accountsApi = createAccountsApi(redwood, history)

  useEffect(async () => {
    const pingIsloggedIn = await accountsApi.checkLogin()

    setIsLoggedIn(pingIsloggedIn)
  }, [])

  if (isLoggedIn) {
    return <Redirect to="/" />
  }

  return (
    <SAccount>
      {/* <SAccountHeader /> */}
      <SAccountCard>
        <SAccountCardHeader>Sign In</SAccountCardHeader>
        <SAccountCardDesc>Always keep your mnemonic safe.</SAccountCardDesc>
        <SAccountCardContent>
          <SignIn
            mnemonic={mnemonic}
            setMnemonic={setMnemonic}
            profileName={profileName}
            setProfileName={setProfileName}
            password={password}
            setPassword={setPassword}
            errorMessage={errorMessage}
            setErrorMessage={setErrorMessage}
            setLoadingText={setLoadingText}
            onSignIn={accountsApi.onSignIn}
          />
        </SAccountCardContent> 
        { loadingText ? <Loading text={loadingText} /> : null }
      </SAccountCard>
    </SAccount>
  )
}

export default Account