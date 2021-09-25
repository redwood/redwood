import React from 'react'
import styled from 'styled-components'

import NormalizeMessage from './../Chat/NormalizeMessage'

const SItemContainer = styled.div`
    display: flex;
    align-items: center;
    padding-top: 8px;
    padding-bottom: 8px;
    background: ${props => props.selected ? props.theme.color.grey[200] : 'transparent'};
    color: ${props => props.selected ? props.theme.color.white : props.theme.color.grey[600]};
`

const SAvatarCircle = styled.div`
    height: 28px;
    width: 28px;
    background: ${props => props.color ? props.color : props.theme.color.indigo[500]};
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 100%;
    margin-left: 12px;

    img {
        height: 18px;
    }
`

const SItemInfo = styled.div`
    display: flex;
    flex-direction: column;
    flex-grow: 1;
    width: calc(100% - 44px);
    padding-left: 8px;
`

const SItemInfoSub = styled.div`
    display: flex;
	justify-content: space-between;
	width: 190.55px;
`

const STime = styled.div`
    padding-right: 12px;
    white-space: nowrap;
	font-size: 0.6rem;
	padding-top: 1px;
    color: ${props => props.theme.color.grey[50]}
`

const ChatName = styled.div`
    font-weight: 500;
	font-size: 12px;
	white-space: nowrap;
    color: ${props => props.selected ? props.theme.color.white : props.theme.color.grey[100]};
`

const MostRecentMessage = styled.div`
    font-size: 12px;
    color: rgba(255, 255, 255, .6);
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow: hidden;
`

function GroupItem({ selected, color, avatar, name, text, time, ...props }) {
	let choppedName = name

	if (name.length > 14) {
		choppedName = `${name.substring(0, 14)}...`
	}

    return (
        <SItemContainer selected={selected} {...props}>
            <SAvatarCircle color={color}>
                <img src={avatar} alt="User Avatar" />
            </SAvatarCircle>
            <SItemInfo selected={selected}>
                <SItemInfoSub>
                    <ChatName selected={selected}>{choppedName}</ChatName>
                    <STime>{time}</STime>
                </SItemInfoSub>
                {/* <MostRecentMessage>{text}</MostRecentMessage> */}
                {/* <MostRecentMessage>
                  <Emoji emoji={'smiley'} size={14}  />
                  <Emoji emoji={'smiley'} size={14}  />
                  <Emoji emoji={'smiley'} size={14}  />
                  <Emoji emoji={'smiley'} size={14}  />
                  <Emoji emoji={'smiley'} size={14}  />
                  dmawl mawld mawldmawlda mwkdl kawmdkl mw
                </MostRecentMessage> */}
                <NormalizeMessage preview msgText={text} selected={selected} />
            </SItemInfo>
        </SItemContainer>
    )
}

export default GroupItem