import React, { useRef, useEffect } from 'react'
import ReactDOM from 'react-dom'
import styled from 'styled-components'
import {
  Editor,
  Range,
} from 'slate'
import { useSlate, ReactEditor } from 'slate-react'
import FormatBoldIcon from '@material-ui/icons/FormatBold';
import FormatItalicIcon from '@material-ui/icons/FormatItalic';
import FormatUnderlinedIcon from '@material-ui/icons/FormatUnderlined';
import StrikethroughSIcon from '@material-ui/icons/StrikethroughS';

const SMenu = styled.div`
    padding: 8px 7px 6px;
    position: absolute;
    z-index: 1;
    top: -10000px;
    left: -10000px;
    margin-top: -6px;
    opacity: 0;
    background-color: #1a1a1f;
    border: 1px solid rgba(255, 255, 255, .12);
    border-radius: 4px;
    transition: opacity 0.75s;
    & > * {
      display: inline-block;
    }
    & > * + * {
      margin-left: 15px;
    }
`

const SFormatButton = styled.div`
    cursor: pointer;
    svg {
      fill: ${props => props.active ? 'white' : 'rgba(255,255,255, .4)'};
    }
`

const Portal = ({ children }) => {
  return typeof document === 'object'
    ? ReactDOM.createPortal(children, document.body)
    : null
}

export const Menu = React.forwardRef(
  (
    { className, ...props },
    ref,
  ) => (
    <SMenu
      {...props}
      ref={ref}
    />
  )
)

const FormatButton = ({ format, icon, toggleFormat, isFormatActive }) => {
  const editor = useSlate()
  return (
    <SFormatButton
      active={isFormatActive(editor, format)}
      onMouseDown={event => {
        event.preventDefault()
        toggleFormat(editor, format)
      }}
    >
      {icon}
    </SFormatButton>
  )
}

const Toolbar = ({ toggleFormat, isFormatActive }) => {
  const ref = useRef()
  const editor = useSlate()

  useEffect(() => {
    const el = ref.current
    const { selection } = editor

    if (!el) {
      return
    }

    if (
      !selection ||
      !ReactEditor.isFocused(editor) ||
      Range.isCollapsed(selection) ||
      Editor.string(editor, selection) === ''
    ) {
      el.removeAttribute('style')
      return
    }

    const domSelection = window.getSelection()
    const domRange = domSelection.getRangeAt(0)
    const rect = domRange.getBoundingClientRect()
    el.style.opacity = '1'
    el.style.top = `${rect.top + window.pageYOffset - el.offsetHeight}px`
    el.style.left = `${rect.left +
      window.pageXOffset -
      el.offsetWidth / 2 +
      rect.width / 2}px`
  })

  return (
    <Portal>
      <SMenu
        ref={ref}
      >
        <FormatButton toggleFormat={toggleFormat} isFormatActive={isFormatActive} format="bold" icon={<FormatBoldIcon />} />
        <FormatButton toggleFormat={toggleFormat} isFormatActive={isFormatActive} format="italic" icon={<FormatItalicIcon />} />
        <FormatButton toggleFormat={toggleFormat} isFormatActive={isFormatActive} format="underlined" icon={<FormatUnderlinedIcon />} />
        <FormatButton toggleFormat={toggleFormat} isFormatActive={isFormatActive} format="strike" icon={<StrikethroughSIcon />} />
      </SMenu>
    </Portal>
  )
}

export default Toolbar