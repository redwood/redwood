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

const ConfirmButtonWrapper = styled.div`
  display: flex;
  justify-content: space-between;
  width: 100%;
`

const ConfirmValueWrapper = styled.div`
  border: 1px solid rgba(255, 255, 255, .18);
  font-size: 12px;
  color: rgba(255, 255, 255, .8);
  line-height: 1.75;
  padding: 6px;
  background: #2f3135;
  position: relative;
  width: 96%;
  span {
    position: absolute;
    top: -20px;
    font-size: 10px;
  }
`

const SErrorMessage = styled.div`
  font-size: 10px;
  color: red;
`

function SignUp(props) {
  return (
    <Fragment>
      { props.errorMessage ? <SErrorMessage>{props.errorMessage}</SErrorMessage> : null}
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
        label={'Password'}
      >
        <Input
          value={props.password}
          onChange={(event) => props.setPassword(event.currentTarget.value)}
          type={'password'}
        />
      </InputLabel>
      <InputLabel
        label={'Confirm Password'}
      >
        <Input
          value={props.confirmPassword}
          onChange={(event) => props.setConfirmPassword(event.currentTarget.value)}
          type={'password'}
        />
      </InputLabel>
      <SLink to={'/profiles'}>Existing profiles.</SLink>
      <SLink to={'/signin'}>Sign into an account.</SLink>
      <Button
        onClick={() => props.confirmProfile(
          props.setErrorMessage,
          props.setLoadingText,
          props.setMnemonic,
          {
            profileName: props.profileName,
            password: props.password,
            confirmPassword: props.confirmPassword,
          }          
        )}
        primary
        style={{ width: '100%', marginTop: 12 }}
        disabled={!(!!props.profileName && !!props.password && !!props.confirmPassword)}
      >Sign Up</Button>
    </Fragment>
  )
}

function ConfirmDisplay(props) {
  return <Fragment>
    { props.errorMessage ? <SErrorMessage>{props.errorMessage}</SErrorMessage> : null}
    <ConfirmValueWrapper style={{ marginBottom: 24 }}>
      <span>Profile Name:</span>     
      {props.profileName}
    </ConfirmValueWrapper>
    <ConfirmValueWrapper>
      <span>Mnemonic:</span>
      {props.mnemonic}
    </ConfirmValueWrapper>
    <ConfirmButtonWrapper>
      <Button
        onClick={() => props.setMnemonic('')}
        primary
        style={{ width: '45%', marginTop: 12 }}
      >Cancel</Button>
      <Button
        onClick={() => props.onSignUp(
          props.setErrorMessage,
          props.setLoadingText,
          {
            profileName: props.profileName,
            mnemonic: props.mnemonic,
            password: props.password,
          }
        )}
        primary
        style={{ width: '45%', marginTop: 12 }}
      >Create</Button>
    </ConfirmButtonWrapper>
  </Fragment>
}

function Account(props) {
  const redwood = useRedwood()
  const history = useHistory()
  const accountsApi = createAccountsApi(redwood, history)
  console.log(accountsApi)

  const [profileName, setProfileName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [mnemonic, setMnemonic] = useState('')
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [loadingText, setLoadingText] = useState('')
  const [errorMessage, setErrorMessage] = useState('')

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
        <SAccountCardHeader>Sign Up</SAccountCardHeader>
        <SAccountCardDesc>{ mnemonic ? 'Please save your mnemonic and keep it secure.' : 'Create an account.'}</SAccountCardDesc>
        <SAccountCardContent>
          { mnemonic ?
            <ConfirmDisplay
              mnemonic={mnemonic}
              setMnemonic={setMnemonic}
              profileName={profileName}
              password={password}
              setLoadingText={setLoadingText}
              errorMessage={errorMessage}
              setErrorMessage={setErrorMessage}
              onSignUp={accountsApi.onSignUp}
            />
          : <SignUp
            setMnemonic={setMnemonic}
            profileName={profileName}
            setProfileName={setProfileName}
            password={password}
            setPassword={setPassword}
            confirmPassword={confirmPassword}
            setConfirmPassword={setConfirmPassword}
            setLoadingText={setLoadingText}
            errorMessage={errorMessage}
            setErrorMessage={setErrorMessage}
            confirmProfile={accountsApi.confirmProfile}
          />          
          }

        </SAccountCardContent>
        { loadingText ? <Loading text={loadingText} /> : null }
      </SAccountCard>
    </SAccount>
  )
}

export default Account
