import React from 'react'
import styled from 'styled-components'

const SItemContainer = styled.div`
  display: flex;
  align-items: center;
  margin-top: 8px;
  margin-bottom: 8px;
  padding-top: 8px;
  padding-bottom: 8px;
  background: ${props => props.selected ? '#2b335c' : 'transparent  '};
`

const SAvatarCircle = styled.div`
  height: 28px;
  width: 28px;
  background: ${props => props.color ? props.color : '#365cd2'};
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 100%;
  margin-left: 12px;
  box-shadow: 0px 2px 1px -1px rgba(0,0,0,0.2), 0px 1px 1px 0px rgba(0,0,0,0.14), 0px 1px 3px 0px rgba(0,0,0,0.12);

  img {
    height: 18px;
  }
`

const SItemInfo = styled.div`
  display: flex;
  justify-content: space-between;
  width: calc(100% - 36px);
  padding-left: 8px;
  span {
    &:first-child {
      font-size: 12px;
      color: white;
    }
    &:nth-child(2) {
      font-size: 8px;
      color: rgba(255, 255, 255, .6);
    }
  }
`

const SItemInfoSub = styled.div`
  display: flex;
  flex-direction: column;
`

const STime = styled.span`
  display: flex;
  align-items: flex-end;
  padding-right: 12px;
`

function GroupItem(props) {
  return (
    <SItemContainer selected={props.selected}>
      <SAvatarCircle color={props.color}>
        <img src={props.avatar} alt="User Avatar" />
      </SAvatarCircle>
      <SItemInfo>
        <SItemInfoSub>
          <span>{props.name}</span>
          <span>{props.text}</span>
        </SItemInfoSub>
        <STime>{props.time}</STime>
      </SItemInfo>
    </SItemContainer>
  )
}

export default GroupItem