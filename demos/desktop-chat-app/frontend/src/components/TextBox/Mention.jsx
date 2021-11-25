import React, { useState, useRef, useEffect } from 'react'
import styled, { keyframes } from 'styled-components'
import { useSelected, useFocused } from 'slate-react'

import UserAvatar from '../UserAvatar'

const animateOnRender = keyframes`
  from {
    transform: scale(0) translateY(-80px);
    opacity: 0;
  }
  to {
    transform: scale(1) translateY(-28px);
    opacity: 1;
  }
`

const SSlateWrapper = styled.span`
    background-color: ${(props) => (props.istargeted ? '#7d8bda' : '#2f3135')};
    color: ${(props) => (props.istargeted ? 'white' : '#7d8bda')};
    vertical-align: baseline;
    display: inline-block;
    border-radius: 4px;
`

const SMention = styled.span`
    padding-left: 2px;
    padding-right: 2px;
    margin: 0 1px;
    vertical-align: baseline;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;

    font-size: 0.9rem;
    user-select: none;
    font-weight: 500;
    cursor: pointer;
    transition: 0.1s all ease-in-out;
    position: relative;

    &:hover {
        color: white;
        background-color: #7d8bda;
    }
`

const SInfoCard = styled.span`
    background: #212123;
    position: ${(props) => (props.absolute ? 'absolute' : 'fixed')};
    z-index: 999;
    padding: 4px 8px;
    transform: translateY(-28px);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    animation: ${animateOnRender} 150ms linear forwards;

    > span {
        font-size: 12px;
        color: white;
        font-weight: 400;
        margin-left: 6px;
        display: flex;
        align-items: center;
        justify-content: center;
        small {
            font-size: 10px;
            color: rgba(255, 255, 255, 0.8);
            margin-left: 2px;
            font-weight: 400;
            white-space: nowrap;
        }
    }
`

// Hook
function useHover() {
    const [value, setValue] = useState(false)
    const ref = useRef(null)
    const handleMouseOver = () => setValue(true)
    const handleMouseOut = () => setValue(false)
    useEffect(
        () => {
            const node = ref.current
            if (node) {
                node.addEventListener('mouseover', handleMouseOver)
                node.addEventListener('mouseout', handleMouseOut)
                return () => {
                    node.removeEventListener('mouseover', handleMouseOver)
                    node.removeEventListener('mouseout', handleMouseOut)
                }
            }
            return () => {}
        },
        [ref.current], // Recall only if ref changes
    )
    return [ref, value]
}

function InfoCard({ user, absolute }) {
    let displayText = (
        <span>
            {user.address.slice(0, 7)} <small>(public key)</small>
        </span>
    )
    if (user.username && !user.nickname) {
        displayText = (
            <span>
                {user.username}
                <small>({user.address.slice(0, 7)})</small>
            </span>
        )
    } else if (user.username && user.nickname) {
        displayText = (
            <span>
                {user.nickname} <small>({user.username})</small>
            </span>
        )
    }

    return (
        <SInfoCard absolute={absolute}>
            <UserAvatar
                mentioninfocard
                style={{ height: 16, width: 16, fontSize: 12 }}
                address={user.address}
            />
            {displayText}
        </SInfoCard>
    )
}

function Mention({
    attributes,
    children,
    element,
    style = {},
    absolute,
    preview,
}) {
    const selected = useSelected()
    const focused = useFocused()
    const [hoverRef, isHovered] = useHover()
    const user = element.selectedUser

    let displayText = user.address.slice(0, 7)

    if (user.username) {
        displayText = user.username
    }

    if (preview) {
        return `@${displayText}`
    }

    return (
        <SSlateWrapper
            {...attributes}
            contentEditable={false}
            istargeted={selected && focused}
        >
            <SMention ref={hoverRef} style={style}>
                @{displayText}
                {(selected && focused) || isHovered ? (
                    <InfoCard user={user} absolute={absolute} />
                ) : null}
            </SMention>
            {children}
        </SSlateWrapper>
    )
}

export default Mention
