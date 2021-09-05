import React from 'react'
import styled from 'styled-components'

import loadingGoo from './assets/loading-goo.svg'
import loadingSvg from './assets/loading.svg'

const SLoading = styled.div`
  background: rgba(0,0,0, .5);
  position: absolute;
  height: 100%;
  width: 100%;
  top: 0px;
  left: 0px;
  border-radius: 4px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  img {
    height: 180px;
  }
  span {
    font-size: 18px;
	color: rgba(255, 255, 255, .8);
	transform: translateY(-28px);
  }
`

function Loading(props) {
  return (
    <SLoading>
      <img alt="Loading" src={loadingGoo} />
      <span>{props.text || 'Loading...'}</span>
    </SLoading>
  )
}

export default Loading