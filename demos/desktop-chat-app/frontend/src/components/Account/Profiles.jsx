import React, { useState, useCallback } from 'react'
import styled from 'styled-components'
import { Link, Redirect } from 'react-router-dom'

import Input, { InputLabel } from '../Input'
import Button from '../Button'
import UserAvatar from '../UserAvatar'
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
    transition: 0.15s ease-in-out all;
    &:hover {
        transform: scale(1.1);
    }
    > span {
        margin-top: 4px;
        color: rgba(255, 255, 255, 0.8);
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
            <UserAvatar address={props.profileName} />
            <span>{props.profileName}</span>
        </SProfile>
    )
}

function SignIn({
    password,
    setPassword,
    profileNames,
    selectedProfile,
    setSelectedProfile,
    errorMessage,
    setErrorMessage,
    setLoadingText,
}) {
    const { login } = useLoginStatus()

    const onSubmitLogin = useCallback(
        async (event) => {
            try {
                event.preventDefault()
                setErrorMessage('')
                setLoadingText('Signing into profile...')
                await login({ profileName: selectedProfile, password })
            } catch (err) {
                setLoadingText('')
                setErrorMessage(err.toString())
            }
        },
        [selectedProfile, password, setErrorMessage, setLoadingText, login],
    )

    return (
        <SAccountCardContent onSubmit={onSubmitLogin}>
            {errorMessage ? (
                <SErrorMessage>{errorMessage}</SErrorMessage>
            ) : null}
            <InputLabel label="Password">
                <Input
                    autoFocus
                    value={password}
                    onChange={(event) => setPassword(event.currentTarget.value)}
                    type="password"
                />
            </InputLabel>
            <SBackProfiles onClick={() => setSelectedProfile('')}>
                Select another profile ({profileNames.length})
            </SBackProfiles>
            <SLink to="/signin">Leave</SLink>
            <Button
                type="submit"
                primary
                style={{ width: '100%', marginTop: 12 }}
                disabled={!password}
            >
                Sign In
            </Button>
        </SAccountCardContent>
    )
}

function SelectProfile(props) {
    return (
        <SAccountCardContent>
            <SSelectProfile>
                <SProfileWrapper>
                    {props.profileNames.length > 0 ? (
                        props.profileNames.map((profileName) => (
                            <Profile
                                key={profileName}
                                onClick={() =>
                                    props.setSelectedProfile(profileName)
                                }
                                profileName={profileName}
                            />
                        ))
                    ) : (
                        <SAccountCardDesc>
                            No profiles to display.
                        </SAccountCardDesc>
                    )}
                </SProfileWrapper>
                <SLink to="/signin">Import existing profile.</SLink>
                <SLink to="/signup">Create a profile.</SLink>
            </SSelectProfile>
        </SAccountCardContent>
    )
}

function Account() {
    const [selectedProfile, setSelectedProfile] = useState('')
    const [password, setPassword] = useState('')
    const [errorMessage, setErrorMessage] = useState('')
    const [loadingText, setLoadingText] = useState('')
    const { isLoggedIn, profileNames } = useLoginStatus()

    if (isLoggedIn) {
        return <Redirect to="/" />
    }

    return (
        <SAccount>
            <SAccountCard>
                <SAccountCardHeader>Profiles</SAccountCardHeader>
                <SAccountCardDesc>
                    {`Profile Name: ${selectedProfile}` || '---'}
                </SAccountCardDesc>
                {selectedProfile ? (
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
                    />
                ) : (
                    <SelectProfile
                        profileNames={profileNames}
                        setSelectedProfile={setSelectedProfile}
                    />
                )}
                {loadingText ? <Loading text={loadingText} /> : null}
            </SAccountCard>
        </SAccount>
    )
}

export default Account
