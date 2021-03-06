import React, { Fragment, useState, useContext } from 'react'
import styled from 'styled-components'
import { Link } from 'react-router-dom'

import Input, { InputLabel } from './../Input'
import Button from './../Button'


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

function SignUp(props) {

  const onSignUp = async () => {
    try {
      console.log('Signned Up')
    } catch (err) {
      console.error(err)
    }
  }

  return (
    <Fragment>
      <InputLabel
        label={'Username'}
      >
        <Input
          value={props.username}
          onChange={(event) => props.setUsername(event.currentTarget.value)}
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
      <SLink to={'/signin'}>Sign into an account.</SLink>
      <Button
        onClick={onSignUp}
        primary
        style={{ width: '100%', marginTop: 12 }}
        disabled={!props.username && !props.password && !props.confirmPassword}
      >Sign In</Button>
    </Fragment>
  )
}

function Account(props) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  return (
    <SAccount>
      {/* <SAccountHeader /> */}
      <SAccountCard>
        <SAccountCardHeader>Sign Up</SAccountCardHeader>
        <SAccountCardDesc>Create an account.</SAccountCardDesc>
        <SAccountCardContent>
          <SignUp
            username={username}
            setUsername={setUsername}
            password={password}
            setPassword={setPassword}
            confirmPassword={confirmPassword}
            setConfirmPassword={setConfirmPassword}
          />
        </SAccountCardContent> 
      </SAccountCard>
    </SAccount>
  )
}

export default Account