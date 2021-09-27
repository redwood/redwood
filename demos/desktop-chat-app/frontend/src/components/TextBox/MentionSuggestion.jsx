import React from 'react'
// import ReactDOM from 'react-dom'
// import styled from 'styled-components'

// import UserAvatar from '../UserAvatar'

// const Portal = ({ children, controlsRef }) => {
//     if (controlsRef) {
//         return ReactDOM.createPortal(children, controlsRef.current)
//     }

//     return typeof document === 'object'
//         ? ReactDOM.createPortal(children, document.body)
//         : null
// }

// const SMentionSuggestion = styled.div`
//     top: -9999px;
//     left: -9999px;
//     position: absolute;
//     width: calc(100% - 36px);
//     z-index: 1;
//     border-radius: 6px;
//     border: 1px solid #555453;
//     padding: 8px;
//     padding-bottom: 12px;
//     padding-top: 12px;
//     background: #222222;
// `

// const SSuggestionHeader = styled.div`
//     width: 100%;
//     border-bottom: 1px solid #555453;
//     padding-bottom: 4px;
//     margin-bottom: 6px;
//     font-size: 12px;
//     font-weight: bold;
// `

// const SSuggestion = styled.div`
//     padding: 1px 3px;
//     border-radius: 3px;
//     color: white;
//     height: 26px;
//     display: flex;
//     align-items: center;
//     justify-content: space-between;
//     background: ${(props) => (props.isSelected ? '#353434' : 'transparent')};
// `

// const SLeftSuggestion = styled.div`
//     display: flex;
//     align-items: center;
//     justify-content: center;
//     padding-left: 4px;
//     span {
//         margin-left: 8px;
//         font-weight: bold;
//         line-height: 2;
//         display: flex;
//         align-items: center;
//     }

//     small {
//         font-size: 10px;
//         color: rgba(255, 255, 255, 0.8);
//         margin-left: 4px;
//         font-weight: 400;
//     }
// `

// const SRightSuggestion = styled.div`
//     margin-right: 12px;
//     span {
//         font-size: 12px;
//         color: white;
//         line-height: 2;
//         display: flex;
//         align-items: center;
//         font-weight: bold;

//         small {
//             font-size: 10px;
//             color: rgba(255, 255, 255, 0.8);
//             margin-left: 4px;
//             font-weight: 400;
//         }
//     }
// `

function MentionSuggestion() {
    return <div>MentionSuggestion</div>
}

// function MentionSuggestion({ mentionUsers, indexMention, controlsRef }) {
//     const mentionRef = useRef()

//     useEffect(() => {
//         if (targetMention && mentionUsers.length > 0) {
//             const el = mentionRef.current
//             const domRange = ReactEditor.toDOMRange(editor, targetMention)
//             const rect = domRange.getBoundingClientRect()
//             const topCalc = 68 + mentionUsers.length * 28

//             el.style.top = `-${topCalc}px`
//             el.style.left = '0px'
//         }
//     }, [
//         mentionUsers.length,
//         editor,
//         indexMention,
//         searchMention,
//         targetMention,
//     ])

//     return (
//         <Portal controlsRef={controlsRef}>
//             <SMentionSuggestion ref={mentionRef}>
//                 <SSuggestionHeader>Users</SSuggestionHeader>
//                 {mentionUsers.map((user, i) => {
//                     let displayText = (
//                         <span>
//                             {user.address.slice(0, 7)}{' '}
//                             <small>(public key)</small>
//                         </span>
//                     )
//                     let trailingText = <span>Test</span>
//                     if (user.username && !user.nickname) {
//                         displayText = <span>{user.username}</span>
//                         trailingText = (
//                             <span>
//                                 {user.address.slice(0, 7)}{' '}
//                                 <small>(public key)</small>
//                             </span>
//                         )
//                     } else if (user.username && user.nickname) {
//                         displayText = (
//                             <span>
//                                 {user.nickname} <small>({user.username})</small>
//                             </span>
//                         )
//                         trailingText = (
//                             <span>
//                                 {user.address.slice(0, 7)}{' '}
//                                 <small>(public key)</small>
//                             </span>
//                         )
//                     }

//                     return (
//                         <SSuggestion
//                             key={user.address}
//                             isSelected={i === indexMention}
//                         >
//                             <SLeftSuggestion>
//                                 <UserAvatar
//                                     mentionsuggestion
//                                     style={{
//                                         height: 20,
//                                         width: 20,
//                                         fontSize: 12,
//                                     }}
//                                     address={user.address}
//                                 />
//                                 {displayText}
//                             </SLeftSuggestion>
//                             <SRightSuggestion>{trailingText}</SRightSuggestion>
//                         </SSuggestion>
//                     )
//                 })}
//             </SMentionSuggestion>
//         </Portal>
//     )
// }

export default MentionSuggestion
