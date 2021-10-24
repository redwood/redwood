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
import UserAvatar from '../UI/UserAvatar'

interface ProfilesProps {
    login: (loginInfo: Record<string, unknown>) => unknown
    isLoggedIn: boolean
    profileNames?: string[]
    connectionError?: unknown
    checkingLogin?: boolean
}

const SProfiles = styled.div`
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
    width: auto;
    min-width: 500px;
    max-width: 700px;
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

const ProfileWrapper = styled.div`
    display: flex;
    width: 100%;
    align-items: center;
    justify-content: center;
    margin-top: 24px;
    flex-wrap: wrap;
`

const SUserAvatar = styled(UserAvatar)`
    margin: 0px !important;
    border-bottom-left-radius: 0px !important;
    border-bottom-right-radius: 0px !important;
`

const SSelectOverlay = styled.div`
    background: rgba(0, 0, 0, 0.6);
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0;
    transition: ${({ theme }) => theme.transition.primary};
    > span {
        width: 80px;
        color: white;
        font-size: 14px;
        text-align: center;
        transition: ${({ theme }) => theme.transition.primary};
    }
`

const SUserAvatarWrapper = styled.div<{ isSmall: boolean }>`
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    border: 1px solid ${({ theme }) => theme.color.accent1};
    border-top-left-radius: 4px;
    border-top-right-radius: 4px;
    margin-right: 16px;
    transition: ${({ theme }) => theme.transition.primary};
    cursor: pointer;
    position: relative;
    margin-bottom: 12px;
    > span {
        color: white;
        font-size: 12px;
        padding-top: 4px;
        padding-bottom: 4px;
    }
    &:hover {
        transform: scale(1.1);
        border-color: ${({ theme }) => theme.color.accent2};
        ${SSelectOverlay} {
            opacity: 1;
            > span {
                font-size: ${({ isSmall }) => (isSmall ? '8px' : '12px')};
            }
        }
    }
`

const SBtnWrapper = styled.div`
    width: 100%;
    display: flex;
    justify-content: space-between;
    align-items: center;
`

function Profiles({
    login = () => false,
    isLoggedIn,
    profileNames = [],
    connectionError = '',
    checkingLogin,
}: ProfilesProps): JSX.Element {
    const [selectedProfile, setSelectedProfile] = useState('')
    const [password, setPassword] = useState('')
    const [errorMessage, setErrorMessage] = useState('')
    const [loadingText, setLoadingText] = useState('')

    const large = useMemo(() => profileNames.length <= 12, [profileNames])

    const onSubmitLogin = useCallback(
        async (event) => {
            try {
                event.preventDefault()
                setErrorMessage('')
                setLoadingText('Signing into profile...')
                await login({ profileName: selectedProfile, password })
            } catch (err: unknown) {
                if (err instanceof Error) {
                    setLoadingText('')
                    setErrorMessage(err.toString())
                } else {
                    setLoadingText('')
                    setErrorMessage(
                        'Error Signing up. Please restart your local node.',
                    )
                }
            }
        },
        [selectedProfile, password, setErrorMessage, setLoadingText, login],
    )

    const content = useMemo(() => {
        if (!selectedProfile) {
            return (
                <>
                    <ProfileWrapper>
                        {profileNames.map((profileName) => (
                            <SUserAvatarWrapper
                                key={profileName}
                                isSmall={!large}
                                onClick={() => setSelectedProfile(profileName)}
                            >
                                <SUserAvatar
                                    large={large}
                                    address={profileName}
                                />
                                <span>{profileName}</span>
                                <SSelectOverlay>
                                    <span>Select Profile</span>
                                </SSelectOverlay>
                            </SUserAvatarWrapper>
                        ))}
                    </ProfileWrapper>
                    <SLinkWrapper>
                        <Link linkTo="/signin">Import Profile</Link>
                        <Link linkTo="/signup">Create Profile</Link>
                    </SLinkWrapper>
                </>
            )
        }

        return (
            <Form onSubmit={onSubmitLogin}>
                <UserAvatar
                    large
                    address={selectedProfile}
                    style={{ marginLeft: 'auto', marginRight: 'auto' }}
                />
                <SBtnWrapper>
                    <Button
                        label="Back"
                        sType="outline"
                        onClick={() => setSelectedProfile('')}
                    />
                    <Button label="Login" sType="primary" type="submit" />
                </SBtnWrapper>
            </Form>
        )
    }, [
        selectedProfile,
        profileNames,
        setSelectedProfile,
        large,
        onSubmitLogin,
    ])

    //     if (checkingLogin) {
    //         return <Redirect to="/loading" />
    //     }

    if (connectionError) {
        return <Redirect to="/connection-error" />
    }

    //     if (isLoggedIn) {
    //         return <Redirect exact to="/" />
    //     }

    return (
        <SProfiles>
            <SCard>
                <SCardContent>
                    <SHushLogo />
                    <SHeader isSmall>Profiles</SHeader>
                    <SP>
                        {selectedProfile
                            ? `Selected Profile: ${selectedProfile}`
                            : 'Existing Profiles'}
                    </SP>
                    {content}
                </SCardContent>
            </SCard>
        </SProfiles>
    )
}

export default Profiles

// const SAccount = styled.section`
//     background-color: ${(props) => props.theme.color.grey[500]};
//     height: 100vh;
//     width: 100vw;
//     display: flex;
//     flex-direction: column;
//     align-items: center;
//     justify-content: center;
// `

// const SAccountCard = styled.div`
//     width: 450px;
//     background: #36393f;
//     box-shadow: 0 2px 10px 0 rgb(0 0 0 / 20%);
//     border-radius: 4px;
//     padding-bottom: 24px;
// `

// const SAccountCardHeader = styled.h2`
//     color: white;
//     font-size: 28px;
//     margin-top: 24px;
//     margin-bottom: 8px;
//     text-align: center;
// `

// const SAccountCardDesc = styled.p`
//     color: rgba(255, 255, 255, 0.8);
//     font-size: 12px;
//     text-align: center;
// `

// const SAccountCardContent = styled.form`
//     display: flex;
//     align-items: center;
//     justify-content: center;
//     flex-direction: column;
//     padding: 16px;
// `

// const SLink = styled(Link)`
//     font-size: 12px;
//     color: #635bff;
//     margin-top: 8px;
// `

// const SBackProfiles = styled.div`
//     font-size: 10px;
//     color: #635bff;
//     margin-top: 8px;
//     text-decoration: underline;
//     cursor: pointer;
// `

// const SProfileWrapper = styled.div`
//     display: flex;
//     align-items: center;
//     justify-content: center;
//     flex-wrap: wrap;
// `

// const SSelectProfile = styled.div`
//     display: flex;
//     flex-direction: column;
//     align-items: center;
//     justify-content: center;
// `

// const SProfile = styled.div`
//     display: flex;
//     align-items: center;
//     justify-content: center;
//     flex-direction: column;
//     margin: 12px;
//     margin-top: 0px;
//     cursor: pointer;
//     transition: 0.15s ease-in-out all;
//     &:hover {
//         transform: scale(1.1);
//     }
//     > span {
//         margin-top: 4px;
//         color: rgba(255, 255, 255, 0.8);
//         font-size: 10px;
//     }
// `

// const SErrorMessage = styled.div`
//     font-size: 10px;
//     color: red;
// `

// function Profile(props) {
//     return (
//         <SProfile onClick={() => props.onClick(props.profileName)}>
//             <UserAvatar address={props.profileName} />
//             <span>{props.profileName}</span>
//         </SProfile>
//     )
// }

// function SignIn({
//     password,
//     setPassword,
//     profileNames,
//     selectedProfile,
//     setSelectedProfile,
//     errorMessage,
//     setErrorMessage,
//     setLoadingText,
//     login,
// }) {
//     const onSubmitLogin = useCallback(
//         async (event) => {
//             try {
//                 event.preventDefault()
//                 setErrorMessage('')
//                 setLoadingText('Signing into profile...')
//                 await login({ profileName: selectedProfile, password })
//             } catch (err) {
//                 setLoadingText('')
//                 setErrorMessage(err.toString())
//             }
//         },
//         [selectedProfile, password, setErrorMessage, setLoadingText, login],
//     )

//     return (
//         <SAccountCardContent onSubmit={onSubmitLogin}>
//             {errorMessage ? (
//                 <SErrorMessage>{errorMessage}</SErrorMessage>
//             ) : null}
//             <InputLabel label="Password">
//                 <Input
//                     autoFocus
//                     value={password}
//                     onChange={(event) => setPassword(event.currentTarget.value)}
//                     type="password"
//                 />
//             </InputLabel>
//             <SBackProfiles onClick={() => setSelectedProfile('')}>
//                 Select another profile ({profileNames.length})
//             </SBackProfiles>
//             <SLink to="/signin">Leave</SLink>
//             <Button
//                 type="submit"
//                 primary
//                 style={{ width: '100%', marginTop: 12 }}
//                 disabled={!password}
//             >
//                 Sign In
//             </Button>
//         </SAccountCardContent>
//     )
// }

// function SelectProfile(props) {
//     return (
//         <SAccountCardContent>
//             <SSelectProfile>
//                 <SProfileWrapper>
//                     {props.profileNames.length > 0 ? (
//                         props.profileNames.map((profileName) => (
//                             <Profile
//                                 key={profileName}
//                                 onClick={() =>
//                                     props.setSelectedProfile(profileName)
//                                 }
//                                 profileName={profileName}
//                             />
//                         ))
//                     ) : (
//                         <SAccountCardDesc>
//                             No profiles to display.
//                         </SAccountCardDesc>
//                     )}
//                 </SProfileWrapper>
//                 <SLink to="/signin">Import existing profile.</SLink>
//                 <SLink to="/signup">Create a profile.</SLink>
//             </SSelectProfile>
//         </SAccountCardContent>
//     )
// }

// function Account({
//     login,
//     isLoggedIn,
//     profileNames,
//     connectionError,
//     checkingLogin,
// }) {
//     const [selectedProfile, setSelectedProfile] = useState('')
//     const [password, setPassword] = useState('')
//     const [errorMessage, setErrorMessage] = useState('')
//     const [loadingText, setLoadingText] = useState('')

//     if (checkingLogin) {
//         return <Redirect to="/loading" />
//     }

//     if (connectionError) {
//         return <Redirect to="/connection-error" />
//     }

//     if (isLoggedIn) {
//         return <Redirect exact to="/" />
//     }

//     return (
//         <SAccount>
//             <SAccountCard>
//                 <SAccountCardHeader>Profiles</SAccountCardHeader>
//                 <SAccountCardDesc>
//                     {`Profile Name: ${selectedProfile}` || '---'}
//                 </SAccountCardDesc>
//                 {selectedProfile ? (
//                     <SignIn
//                         password={password}
//                         setPassword={setPassword}
//                         selectedProfile={selectedProfile}
//                         profileNames={profileNames}
//                         setSelectedProfile={setSelectedProfile}
//                         errorMessage={errorMessage}
//                         setErrorMessage={setErrorMessage}
//                         loadingText={loadingText}
//                         setLoadingText={setLoadingText}
//                         login={login}
//                     />
//                 ) : (
//                     <SelectProfile
//                         profileNames={profileNames}
//                         setSelectedProfile={setSelectedProfile}
//                     />
//                 )}
//                 {loadingText ? <Loading text={loadingText} /> : null}
//             </SAccountCard>
//         </SAccount>
//     )
// }

// export default Account
