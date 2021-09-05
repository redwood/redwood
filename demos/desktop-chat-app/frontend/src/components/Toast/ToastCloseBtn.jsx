import React from 'react'
import styled from 'styled-components'

import cancelIcon from './../../assets/cancel.svg'

const SCloseBtnContainer = styled.div`
	height: 16px;
	width: 16px;
	cursor: pointer;
	img {
		height: 14px;
		width: 12px;
	}
`

const ToastCloseBtn = ({ closeToast }) => (
	<SCloseBtnContainer className="Toastify__close-button Toastify__close-button--light" onClick={closeToast}>
		<img src={cancelIcon} alt="Cancel Icon" />
	</SCloseBtnContainer>
)

export default ToastCloseBtn