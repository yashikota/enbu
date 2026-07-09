'use client'
import { forwardRef } from 'react'
import type { HTMLStyledProps } from 'styled-system/jsx'
import { Span } from './span'
import { Spinner } from './spinner'

export interface LoaderProps extends HTMLStyledProps<'span'> {
  visible?: boolean
  spinner?: React.ReactNode
  spinnerPlacement?: 'start' | 'end'
  text?: React.ReactNode
  children?: React.ReactNode
}

export const Loader = forwardRef<HTMLSpanElement, LoaderProps>(function Loader(props, ref) {
  const {
    spinner = <Spinner size="sm" borderWidth="0.125em" color="inherit" />,
    spinnerPlacement = 'start',
    children,
    text,
    visible = true,
    ...rest
  } = props

  if (!visible) return <>{children}</>

  if (text) {
    return (
      <Span ref={ref} display="contents" {...rest}>
        {spinnerPlacement === 'start' && spinner}
        {text}
        {spinnerPlacement === 'end' && spinner}
      </Span>
    )
  }

  if (spinner) {
    return (
      <Span ref={ref} display="inline-flex" alignItems="center" justifyContent="center" position="relative" {...rest}>
        {spinner}
        <Span visibility="hidden" display="contents">
          {children}
        </Span>
      </Span>
    )
  }

  return (
    <Span ref={ref} display="contents" {...rest}>
      {children}
    </Span>
  )
})
