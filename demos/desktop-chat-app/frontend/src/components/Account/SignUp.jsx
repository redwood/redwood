import React, { useState, useCallback } from 'react'
import styled from 'styled-components'
import { Link, Redirect } from 'react-router-dom'

import Input, { InputLabel } from '../Input'
import Button from '../Button'
import Loading from './Loading'
import useLoginStatus from '../../hooks/useLoginStatus'

const SAccount = styled.section`
    background-color: ${(props) => props.theme.color.grey[500]};
    height: 100vh;
    width: 100vw;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
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
    color: rgba(255, 255, 255, 0.8);
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

const ConfirmButtonWrapper = styled.div`
    display: flex;
    justify-content: space-between;
    width: 100%;
`

const ConfirmValueWrapper = styled.div`
    border: 1px solid rgba(255, 255, 255, 0.18);
    font-size: 12px;
    color: rgba(255, 255, 255, 0.8);
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

function SignUp({
    profileNames,
    setMnemonic,
    profileName,
    setProfileName,
    password,
    setPassword,
    confirmPassword,
    setConfirmPassword,
    setLoadingText,
    errorMessage,
    setErrorMessage,
}) {
    const { signup } = useLoginStatus()

    const onSubmitSignUp = useCallback(
        async (event) => {
            event.preventDefault()
            setErrorMessage('')
            setLoadingText('Creating profile...')
            try {
                if (password !== confirmPassword) {
                    setErrorMessage('Passwords do not match.')
                    setLoadingText('')
                    return
                }
                const mnemonic = await signup({ profileName })
                setMnemonic(mnemonic)
                setLoadingText('')
                setErrorMessage('')
            } catch (err) {
                setLoadingText('')
                setErrorMessage(err.toString())
            }
        },
        [
            profileName,
            password,
            confirmPassword,
            setMnemonic,
            setErrorMessage,
            setLoadingText,
        ],
    )

    return (
        <SAccountCardContent onSubmit={onSubmitSignUp}>
            {errorMessage ? (
                <SErrorMessage>{errorMessage}</SErrorMessage>
            ) : null}
            <InputLabel label="Profile Name">
                <Input
                    value={profileName}
                    onChange={(event) =>
                        setProfileName(event.currentTarget.value)
                    }
                    type="text"
                    autoFocus
                />
            </InputLabel>
            <InputLabel label="Password">
                <Input
                    value={password}
                    onChange={(event) => setPassword(event.currentTarget.value)}
                    type="password"
                />
            </InputLabel>
            <InputLabel label="Confirm Password">
                <Input
                    value={confirmPassword}
                    onChange={(event) =>
                        setConfirmPassword(event.currentTarget.value)
                    }
                    type="password"
                />
            </InputLabel>
            <SLink to="/profiles">
                Existing Profiles ({profileNames.length}).
            </SLink>
            <SLink to="/signin">Import existing profile.</SLink>
            <Button
                type="submit"
                primary
                style={{ width: '100%', marginTop: 12 }}
                disabled={!(!!profileName && !!password && !!confirmPassword)}
            >
                Sign Up
            </Button>
        </SAccountCardContent>
    )
}

function ConfirmDisplay({
    mnemonic,
    setMnemonic,
    profileName,
    password,
    setLoadingText,
    errorMessage,
    setErrorMessage,
}) {
    const { login } = useLoginStatus()

    const onClickLogin = useCallback(async () => {
        try {
            setErrorMessage('')
            setLoadingText('Validating and generating mnemonic...')
            await login({ profileName, mnemonic, password })
            setLoadingText('')
        } catch (err) {
            setLoadingText('')
            setErrorMessage(err.toString())
        }
    }, [
        profileName,
        mnemonic,
        password,
        setErrorMessage,
        setLoadingText,
        login,
    ])

    return (
        <SAccountCardContent>
            {errorMessage ? (
                <SErrorMessage>{errorMessage}</SErrorMessage>
            ) : null}
            <ConfirmValueWrapper style={{ marginBottom: 24 }}>
                <span>Profile Name:</span>
                {profileName}
            </ConfirmValueWrapper>
            <ConfirmValueWrapper>
                <span>Mnemonic:</span>
                {mnemonic}
            </ConfirmValueWrapper>
            <ConfirmButtonWrapper>
                <Button
                    onClick={() => setMnemonic('')}
                    primary
                    style={{ width: '45%', marginTop: 12 }}
                >
                    Cancel
                </Button>
                <Button
                    onClick={onClickLogin}
                    primary
                    style={{ width: '45%', marginTop: 12 }}
                >
                    Create
                </Button>
            </ConfirmButtonWrapper>
        </SAccountCardContent>
    )
}

function Account(props) {
    const { isLoggedIn } = useLoginStatus()

    const [profileName, setProfileName] = useState('')
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [mnemonic, setMnemonic] = useState('')
    const [loadingText, setLoadingText] = useState('')
    const [errorMessage, setErrorMessage] = useState('')

    if (isLoggedIn) {
        return <Redirect to="/" />
    }

    return (
        <SAccount>
            <SAccountCard>
                <SAccountCardHeader>Sign Up</SAccountCardHeader>
                <SAccountCardDesc>
                    {mnemonic
                        ? 'Please save your mnemonic and keep it secure.'
                        : 'Create a profile.'}
                </SAccountCardDesc>
                {mnemonic ? (
                    <ConfirmDisplay
                        mnemonic={mnemonic}
                        setMnemonic={setMnemonic}
                        profileName={profileName}
                        password={password}
                        setLoadingText={setLoadingText}
                        errorMessage={errorMessage}
                        setErrorMessage={setErrorMessage}
                    />
                ) : (
                    <SignUp
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
                        profileNames={props.profileNames || []}
                    />
                )}
                {loadingText ? <Loading text={loadingText} /> : null}
            </SAccountCard>
        </SAccount>
    )
}

export default Account
