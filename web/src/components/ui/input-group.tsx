'use client'
import { type ReactNode, forwardRef } from 'react'
import { styled } from 'styled-system/jsx'

const Root = styled('div', {
  base: {
    display: 'flex',
    alignItems: 'center',
    position: 'relative',
    width: 'full',
  },
})

const Element = styled('div', {
  base: {
    position: 'absolute',
    display: 'flex',
    alignItems: 'center',
    pointerEvents: 'none',
    zIndex: 1,
  },
})

export interface InputGroupProps extends React.ComponentProps<typeof Root> {
  startElement?: ReactNode
  endElement?: ReactNode
  children?: ReactNode
}

export const InputGroup = forwardRef<HTMLDivElement, InputGroupProps>(
  function InputGroup(props, ref) {
    const { startElement, endElement, children, ...rest } = props

    return (
      <Root ref={ref} {...rest}>
        {startElement && (
          <Element style={{ left: 0, top: 0, height: '100%', paddingLeft: '8px' }}>
            {startElement}
          </Element>
        )}
        {children}
        {endElement && (
          <Element style={{ right: 0, top: 0, height: '100%', paddingRight: '8px' }}>
            {endElement}
          </Element>
        )}
      </Root>
    )
  },
)
