import React, { Fragment, useState, useEffect, useCallback } from 'react'
import styled from 'styled-components'
import { Link, Redirect } from 'react-router-dom'

import Input, { InputLabel } from './../Input'
import Button from './../Button'
import Loading from './Loading'
import useLoginStatus from '../../hooks/useLoginStatus'

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

const SAccountCardContent = styled.form`
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    padding: 16px;
`

const SLink = styled(Link)`
    font-size: 12px;
    color: #635bff;
    margin-top: 8px;
`

const SErrorMessage = styled.div`
    font-size: 10px;
    color: red;
`

function SignIn({ profileNames, mnemonic, setMnemonic, profileName, setProfileName, password, setPassword, errorMessage, setErrorMessage, setLoadingText }) {
	let { login } = useLoginStatus()

    let onSubmitLogin = useCallback(async (event) => {
		event.preventDefault()
        setErrorMessage('')
        setLoadingText('Validating and generating mnemonic...')
        try {
            await login({ profileName, mnemonic, password })
            setLoadingText('')
        } catch (err) {
            setLoadingText('')
            setErrorMessage(err.toString())
        }
    }, [profileName, mnemonic, password, setErrorMessage, setLoadingText, login])

    return (
		<SAccountCardContent onSubmit={onSubmitLogin}>
            {errorMessage ? <SErrorMessage>{errorMessage}</SErrorMessage> : null}
            <InputLabel label={'Profile Name'}>
                <Input
                    value={profileName}
                    onChange={(event) => setProfileName(event.currentTarget.value)}
                    type={'text'}
                    autoFocus
                />
            </InputLabel>
            <InputLabel label={'Mnemonic'}>
                <Input
                    value={mnemonic}
                    onChange={(event) => setMnemonic(event.currentTarget.value)}
                    type={'password'}
                />
            </InputLabel>
            <InputLabel label={'Password'}>
                <Input
                    value={password}
                    onChange={(event) => setPassword(event.currentTarget.value)}
                    type={'password'}
                />
            </InputLabel>
            <SLink to={'/profiles'}>Existing Profiles ({profileNames.length}).</SLink>
            <SLink to={'/signup'}>Create a profile.</SLink>
            <Button
				primary
				style={{ width: '100%', marginTop: 12 }}
				disabled={!(!!mnemonic && !!profileName && !!password)}
				type="submit"
			>Sign In</Button>
		</SAccountCardContent>
    )
}

function Account(props) {
    const [mnemonic, setMnemonic] = useState('')
    const [profileName, setProfileName] = useState('')
    const [password, setPassword] = useState('')
    const [errorMessage, setErrorMessage] = useState('')
    const [loadingText, setLoadingText] = useState('')
    const { isLoggedIn } = useLoginStatus()

    if (isLoggedIn) {
        return <Redirect to="/" />
    }

    return (
        <SAccount>
            {/* <SAccountHeader /> */}
            <SAccountCard>
                <SAccountCardHeader>Import Existing Profile</SAccountCardHeader>
                <SAccountCardDesc>Always keep your mnemonic safe.</SAccountCardDesc>
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
						profileNames={props.profileNames || []}
                    />
                { loadingText ? <Loading text={loadingText} /> : null }
            </SAccountCard>
        </SAccount>
    )
}

export default Account