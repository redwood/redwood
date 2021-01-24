import React, { useState, useCallback } from 'react'
import styled from 'styled-components'

import userPlaceholderImg from './assets/user_placeholder.png'

const SUserControlContainer = styled.div`
    display: flex;
    align-items: center;
    height: 56px;
    width: 100%;
    background-color: ${props => props.theme.color.grey[500]};
`

const SUserLeft = styled.div`
    flex: 2;
    display: flex;
    align-items: center;
    padding-left: 12px;
    transition: .15s ease-in-out all;
    height: 100%;
    cursor: pointer;

    img {
        height: 28px;
    }

    &:hover {
        background: #2d3354;
    }
`

const SUsernameWrapper = styled.div`
    display: flex;
    flex-direction: column;
    margin-left: 8px;
    span {
        color: white;
        &:first-child {
            font-size: 14px;
        }
        &:nth-child(2) {
            font-size: 10px;
            color: rgba(255, 255, 255, .6);
            font-weight: 300;
        }
    }
`

function UserControl({ server }) {
    return (
        <SUserControlContainer>
            <SUserLeft>
                <img src={userPlaceholderImg} alt="User Avatar" />
                <SUsernameWrapper>
                    <span>Tim Shenk</span>
                    <span>@stenkatron_69</span>
                </SUsernameWrapper>
            </SUserLeft>
        </SUserControlContainer>
    )
}

export default UserControl