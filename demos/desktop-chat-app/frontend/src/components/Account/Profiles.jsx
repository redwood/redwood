import React, { Fragment, useState, useEffect } from 'react'
import styled from 'styled-components'
import { Link, useHistory } from 'react-router-dom'
import * as RedwoodReact from 'redwood.js/dist/module/react'

import Input, { InputLabel } from './../Input'
import Button from './../Button'
import UserAvatar from './../UserAvatar'

const { useRedwood } = RedwoodReact

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
      const isLoggedIn = await checkLogin()

      if (isLoggedIn) {
        const logoutResp = await fetch('http://localhost:54231/api/logout', { method: 'POST' })
        console.log(logoutResp)
        console.log('LOGGED OUT')
        if (logoutResp.status === 500) {
          console.log(logoutResp)
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
        console.log(errorText)
      }
      
      await redwood.fetchIdentities(redwood.redwoodClient)
      history.push('/')
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <Fragment>
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
        onClick={onSignIn}
        primary
        style={{ width: '100%', marginTop: 12 }}
        disabled={!props.password}
      >Sign In</Button>
    </Fragment>
  )
}

function SelectProfile(props) {
  return (
    <SProfileWrapper>
      { props.profileNames.map((profileName, key) => {
        return (
          <Profile
            key={key}
            onClick={() => props.setSelectedProfile(profileName)}
            profileName={profileName}
          />
        )
      })}
    </SProfileWrapper>
  )
}

function Account(props) {
  const [selectedProfile, setSelectedProfile] = useState('')
  const [password, setPassword] = useState('')
  const [profileNames, setProfileNames] = useState([])

  const getProfileNames = async () => {
    try {
      let resp = await (await fetch('http://localhost:54231/api/profile-names', { method: 'GET' })).json()
      setProfileNames(resp.profileNames)
    } catch (err) {
      return err
    }
  }

  useEffect(() => {
    getProfileNames()
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
            />
          : <SelectProfile
              profileNames={profileNames}
              setSelectedProfile={setSelectedProfile}
          />}
        </SAccountCardContent> 
      </SAccountCard>
    </SAccount>
  )
}

export default Account