import React, { useState, useCallback } from 'react'

import Button from '../Button'
import Input, { InputLabel } from '../Input'
import UploadAvatar from '../UploadAvatar'
import { Pane, PaneContent, PaneActions } from '../SlidingPane'

function NameAndIconPane({
    setRequestValues,
    onClickBack,
    onClickNext,
    activeStep,
    ...props
}) {
    const [serverName, setServerName] = useState('')
    const [iconImg, setIconImg] = useState(null)
    const [iconFile, setIconFile] = useState(null)

    const onChangeServerName = useCallback(
        (e) => {
            setServerName(e.target.value)
        },
        [setServerName],
    )

    const handleBack = useCallback(() => {
        setRequestValues((prev) => ({ ...prev, serverName, iconImg, iconFile }))
        onClickBack()
    }, [setRequestValues, onClickBack, serverName, iconImg, iconFile])

    const handleNext = useCallback(() => {
        setRequestValues((prev) => ({ ...prev, serverName, iconImg, iconFile }))
        onClickNext()
    }, [setRequestValues, onClickNext, serverName, iconImg, iconFile])

    const handleEnter = (e) => {
        if (e.keyCode === 13 && activeStep === 0) {
            handleNext()
        }
    }

    return (
        <Pane {...props}>
            <PaneContent>
                <UploadAvatar
                    iconImg={iconImg}
                    setIconImg={setIconImg}
                    setIconFile={setIconFile}
                />
                <InputLabel label="Server Name">
                    <Input
                        value={serverName}
                        onChange={onChangeServerName}
                        width="100%"
                        autoFocus
                        onKeyDown={handleEnter}
                    />
                </InputLabel>
            </PaneContent>

            <PaneActions>
                <Button onClick={handleBack}>Back</Button>
                <Button
                    onClick={handleNext}
                    primary
                    disabled={(serverName || '').trim().length === 0}
                >
                    Next
                </Button>
            </PaneActions>
        </Pane>
    )
}

export default NameAndIconPane
