import React from 'react'
import styled from 'styled-components'

import UserControl from './UserControl'
import GroupItem from './GroupItem'

import avatarPlaceholder from './assets/avatar.png'
import placeholder2 from './assets/placeholder2.png'
import placeholder3 from './assets/placeholder3.png'
import placeholder4 from './assets/placeholder4.png'

const SSidebarContainer = styled.div`
  display: flex;
  flex-direction: column;
  width: 200px;
  background: #1b203c;
  box-shadow: 0px 3px 3px -2px rgba(0,0,0,0.2), 0px 3px 4px 0px rgba(0,0,0,0.14), 0px 1px 8px 0px rgba(0,0,0,0.12);
`

const SGroupTitle = styled.div`
  font-size: 12px;
  color: rgba(255, 255, 255, .6);
  font-weight: 300;
  margin-top: 24px;
  padding-left: 12px;
`

function RedSidebar () {

  return (
    <SSidebarContainer>
      <UserControl />
      <SGroupTitle>Groups (3)</SGroupTitle>
      <GroupItem
        selected
        name={'Chainlink Chat'}
        text={`I'm a moon boy that's...`}
        time={'8 min ago'}
        avatar={avatarPlaceholder}
      />
      <GroupItem
        name={'Just Memein'}
        text={`Kek I like to meme as we...`}
        time={'3:54 PM'}
        color={'#f6471b'}
        avatar={placeholder2}
      />
      <GroupItem
        name={'The Boyz'}
        text={`So when are we going to...`}
        time={'2:12 PM'}
        color={'#9945a1'}
        avatar={placeholder3}
      />
      <SGroupTitle>Friends (1)</SGroupTitle>
      <GroupItem
        name={'Pepe'}
        text={`I am your God now...`}
        time={'8 min ago'}
        color={'#f6a8f4'}
        avatar={placeholder4}
      />
    </SSidebarContainer>
  )
}

export default RedSidebar