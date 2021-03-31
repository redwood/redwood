import React, { Fragment, useState, useEffect } from 'react'
import styled from 'styled-components'
import { Link, useHistory } from 'react-router-dom'
import { useRedwood } from 'redwood/dist/main/react'

import Input, { InputLabel } from './../Input'
import Button from './../Button'
import UserAvatar from './../UserAvatar'
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

const SBackProfiles = styled.div`
  font-size: 10px;
  color: #635bff;
  margin-top: 8px;
  text-decoration: underline;
  cursor: pointer;
`

const SProfileWrapper = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
`

const SSelectProfile = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
`

const SProfile = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  margin: 12px;
  margin-top: 0px;
  cursor: pointer;
  transition: .15s ease-in-out all;
  &:hover {
    transform: scale(1.1);
  }
  > span {
    margin-top: 4px;
    color: rgba(255, 255, 255, .8);
    font-size: 10px;
  }
`

const SErrorMessage = styled.div`
  font-size: 10px;
  color: red;
`

function Profile(props) {
  return (
    <SProfile onClick={() => props.onClick(props.profileName)}>
      <UserAvatar username={props.profileName} />
      <span>{props.profileName}</span>
    </SProfile>
  )
}

function SignIn(props) {
  const redwood = useRedwood()
  const history = useHistory()

  const checkLogin = async () => {
    try {
      let resp = await fetch('http://localhost:54231/api/check-login', { method: 'POST' })

      const jsonResp = await resp.text()

      return jsonResp === 'true'
    } catch (err) {
      console.error(err)
    }
  }

  const onSignIn = async () => {
    try {
      props.setErrorMessage('')
      props.setLoadingText('Signing into profile...')
      const isLoggedIn = await checkLogin()

      if (isLoggedIn) {
        const logoutResp = await fetch('http://localhost:54231/api/logout', { method: 'POST' })
        console.log(logoutResp)
        console.log('LOGGED OUT')
        if (logoutResp.status === 500) {
          props.setErrorMessage(logoutResp)
          props.setLoadingText('')
          return
        }
      }

      let resp = await fetch('http://localhost:54231/api/login', {
        method: 'POST',
        body: JSON.stringify({
          profileName: props.selectedProfile,
          password: props.password,
        }),
      })

      if (resp.status === 500) {
        const errorText = await resp.text()
        props.setErrorMessage(errorText)
        props.setLoadingText('')
        return
        console.log(errorText)
      }
      
      await redwood.fetchIdentities(redwood.redwoodClient)
      history.push('/')
    } catch (err) {
      props.setErrorMessage('')
      props.setLoadingText('')
      console.error(err)
    }
  }

  return (
    <Fragment>
      { props.errorMessage ? <SErrorMessage>{props.errorMessage}</SErrorMessage> : null}
      <InputLabel
        label={'Password'}
      >
        <Input
          value={props.password}
          onChange={(event) => props.setPassword(event.currentTarget.value)}
          type={'password'}
        />
      </InputLabel>
      <SBackProfiles onClick={() => props.setSelectedProfile('')}>Select another profile ({props.profileNames.length})</SBackProfiles>
      <SLink to={'/signin'}>Leave</SLink>
      <Button
        onClick={() => props.signInProfile(
          props.setErrorMessage,
          props.setLoadingText,
          {
            selectedProfile: props.selectedProfile,
            password: props.password, 
          }
        )}
        primary
        style={{ width: '100%', marginTop: 12 }}
        disabled={!props.password}
      >Sign In</Button>
    </Fragment>
  )
}

function SelectProfile(props) {
  return (
    <SSelectProfile>
      <SProfileWrapper>
        {props.profileNames.length > 0 ? props.profileNames.map((profileName, key) => {
          return (
            <Profile
              key={key}
              onClick={() => props.setSelectedProfile(profileName)}
              profileName={profileName}
            />
          )
        }) : <SAccountCardDesc>No profiles to display.</SAccountCardDesc>}
      </SProfileWrapper>
      <SLink to={'/signin'}>Sign into account.</SLink>
      <SLink to={'/signup'}>Create an account.</SLink>
    </SSelectProfile>
  )
}

function Account(props) {
  const redwood = useRedwood()
  const history = useHistory()

  const [selectedProfile, setSelectedProfile] = useState('')
  const [password, setPassword] = useState('')
  const [profileNames, setProfileNames] = useState([])
  const [errorMessage, setErrorMessage] = useState('')
  const [loadingText, setLoadingText] = useState('')
  const accountsApi = createAccountsApi(redwood, history)

  useEffect(async () => {
    const pns = await accountsApi.getProfileNames()
    setProfileNames(pns)
  }, [])

  return (
    <SAccount>
      <SAccountCard>
        <SAccountCardHeader>Profiles</SAccountCardHeader>
        <SAccountCardDesc>{ `Profile Name: ${selectedProfile}` || '---'}</SAccountCardDesc>
        <SAccountCardContent>
          { selectedProfile ? 
            <SignIn
              password={password}
              setPassword={setPassword}
              selectedProfile={selectedProfile}
              profileNames={profileNames}
              setSelectedProfile={setSelectedProfile}
              errorMessage={errorMessage}
              setErrorMessage={setErrorMessage}
              loadingText={loadingText}
              setLoadingText={setLoadingText}
              signInProfile={accountsApi.signInProfile}
            />
          : <SelectProfile
              profileNames={profileNames}
              setSelectedProfile={setSelectedProfile}
          />}
        </SAccountCardContent> 
        { loadingText ? <Loading text={loadingText} /> : null }
      </SAccountCard>
    </SAccount>
  )
}

export default Account