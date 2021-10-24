import { useState, useCallback, useMemo } from 'react'
import styled from 'styled-components'
import { Redirect } from 'react-router-dom'

import Link from '../UI/Link'
import Header from '../UI/Text/Header'
import Input from '../UI/Input'
import Card from '../UI/Card'
import P from '../UI/Text/P'
import Button from '../UI/Button'
import HushLogo from '../UI/HushLogo'
import ErrorDisplay from '../UI/Text/ErrorDisplay'
import LoadingSection from '../UI/Loading/LoadingSection'

interface SignUpProps {
    isLoggedIn: boolean
    signup?: (signupInfo: Record<string, unknown>) => string
    profileNames: string[]
    login?: (loginInfo: Record<string, unknown>) => unknown
    connectionError?: string
    checkingLogin?: boolean
}

const SSignUp = styled.div`
    background: ${({ theme }) => theme.color.textDark};
    width: 100vw;
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
`

const SCard = styled(Card)`
    box-sizing: border-box;
    padding: 12px;
    width: 500px;
    height: auto;
    position: relative;
`

const SCardContent = styled.div``

const SHeader = styled(Header)`
    font-weight: 500;
    text-align: center;
`

const SP = styled(P)`
    text-align: center;
`

const Form = styled.form`
    display: flex;
    flex-direction: column;
    width: 100%;
    > div {
        margin-bottom: 12px;
    }
`

const SButton = styled(Button)`
    width: 100%;
    margin-top: 18px !important;
`

const SLinkWrapper = styled.div`
    display: flex;
    align-items: center;
    justify-content: center;
    flex-direction: column;
    margin-top: 24px;
    margin-bottom: 12px;
`

const SHushLogo = styled(HushLogo)`
    position: absolute;
    top: 12px;
    left: 12px;
    height: 24px;
`

const SDisplayWell = styled.div``
const SDisplayWellLabel = styled.span`
    color: ${({ theme }) => theme.color.text};
    font-size: 14px;
`
const SDisplayWellText = styled.div`
    border: 1px solid ${({ theme }) => theme.color.accent2};
    color: ${({ theme }) => theme.color.accent2};
    border-radius: 2px;
    font-size: 14px;
    padding: 8px 12px;
    margin-top: 12px;
`

const ConfirmWrapper = styled.div`
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 32px;
`

interface DisplayWellProps {
    label: string
    text: string
}

function DisplayWell({ label = '', text = '' }: DisplayWellProps): JSX.Element {
    return (
        <SDisplayWell>
            <SDisplayWellLabel>{label}</SDisplayWellLabel>
            <SDisplayWellText>{text}</SDisplayWellText>
        </SDisplayWell>
    )
}

function SignUp({
    isLoggedIn,
    checkingLogin,
    signup = () => '',
    profileNames = [],
    login = () => false,
    connectionError = '',
}: SignUpProps): JSX.Element {
    const [profileName, setProfileName] = useState('')
    const [password, setPassword] = useState('')
    const [confirmPassword, setConfirmPassword] = useState('')
    const [mnemonic, setMnemonic] = useState('')
    const [loadingText, setLoadingText] = useState('')
    const [errorMessage, setErrorMessage] = useState<Record<string, string>>({})

    const isDisabled = useMemo(
        () => !(!!profileName && !!password && !!confirmPassword),
        [profileName, password, confirmPassword],
    )

    const onSubmitSignUp = useCallback(
        async (event) => {
            event.preventDefault()
            setErrorMessage({})
            setLoadingText('Validating and generating mnemonic...')
            try {
                if (password !== confirmPassword) {
                    setErrorMessage({
                        password: 'Passwords do not match.',
                        confirmPassword: 'Passwords do not match.',
                    })
                    setLoadingText('')
                    return
                }
                const generatedMnemonic: string = await signup({ profileName })
                setMnemonic(generatedMnemonic)
                setLoadingText('')
                setErrorMessage({})
            } catch (err: unknown) {
                if (err instanceof Error) {
                    setLoadingText('')
                    setErrorMessage({ general: err.toString() })
                } else {
                    setLoadingText('')
                    setErrorMessage({
                        general:
                            'Error Signing up. Please restart your local node.',
                    })
                }
            }
        },
        [
            profileName,
            password,
            confirmPassword,
            setMnemonic,
            setErrorMessage,
            setLoadingText,
            signup,
        ],
    )

    const onClickLogin = useCallback(async () => {
        try {
            setErrorMessage({})
            setLoadingText('Creating profile...')
            await login({ profileName, mnemonic, password })
            setLoadingText('')
        } catch (err: unknown) {
            if (err instanceof Error) {
                setLoadingText('')
                setErrorMessage({ general: err.toString() })
            } else {
                setLoadingText('')
                setErrorMessage({
                    general:
                        'Error Signing up. Please restart your local node.',
                })
            }
        }
    }, [
        profileName,
        mnemonic,
        password,
        setErrorMessage,
        setLoadingText,
        login,
    ])

    const content = useMemo(() => {
        if (!mnemonic) {
            return (
                <Form onSubmit={onSubmitSignUp}>
                    <Input
                        id="profileName"
                        value={profileName}
                        onChange={(e) => setProfileName(e.currentTarget.value)}
                        width="100%"
                        label="Profile Name"
                        type="text"
                        errorText={errorMessage.general}
                        placeholder="Enter profile name..."
                    />
                    <Input
                        id="password"
                        value={password}
                        onChange={(e) => setPassword(e.currentTarget.value)}
                        width="100%"
                        label="Password"
                        type="password"
                        errorText={errorMessage.password}
                        placeholder="Enter password..."
                    />
                    <Input
                        id="confirmPassword"
                        value={confirmPassword}
                        onChange={(e) =>
                            setConfirmPassword(e.currentTarget.value)
                        }
                        width="100%"
                        label="Confirm Password"
                        type="password"
                        errorText={errorMessage.confirmPassword}
                        placeholder="Enter confirm password..."
                    />
                    <SButton
                        label="Sign Up"
                        type="submit"
                        sType="primary"
                        disabled={isDisabled}
                    />
                    <SLinkWrapper>
                        <Link linkTo="/profiles">
                            Existing Profiles ({profileNames.length})
                        </Link>
                        <Link linkTo="/signin">Import Profile</Link>
                    </SLinkWrapper>
                </Form>
            )
        }

        return (
            <Form onSubmit={onClickLogin}>
                <DisplayWell label="Profile Name" text={profileName} />
                <DisplayWell label="Mnemonic" text={mnemonic} />
                <ConfirmWrapper>
                    <Button
                        sType="outline"
                        label="Cancel"
                        onClick={() => setMnemonic('')}
                    />
                    <Button
                        sType="primary"
                        label="Create"
                        onClick={onClickLogin}
                    />
                </ConfirmWrapper>
            </Form>
        )
    }, [
        mnemonic,
        confirmPassword,
        password,
        profileNames,
        profileName,
        isDisabled,
        onClickLogin,
        errorMessage,
        onSubmitSignUp,
    ])

    // if (checkingLogin) {
    //     return <Redirect to="/loading" />
    // }

    // if (connectionError) {
    //     return <Redirect to="/connection-error" />
    // }

    // if (isLoggedIn) {
    //     return <Redirect exact to="/" />
    // }

    return (
        <SSignUp>
            <SCard>
                <SCardContent>
                    <SHushLogo />
                    <SHeader isSmall>Sign Up</SHeader>
                    {errorMessage.general ? (
                        <ErrorDisplay>{errorMessage.general}</ErrorDisplay>
                    ) : (
                        <SP>
                            {!mnemonic
                                ? 'Create a Profile'
                                : 'Please save your mnemonic and keep it secure.'}
                        </SP>
                    )}
                    {content}
                    <LoadingSection
                        isLoading={!!loadingText}
                        loadingText={loadingText}
                    />
                </SCardContent>
            </SCard>
        </SSignUp>
    )
}

export default SignUp
