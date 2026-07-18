import type { ComponentProps } from 'react'
import { styled } from 'styled-system/jsx'

export type SeparatorProps = ComponentProps<'hr'>

export const Separator = styled('hr', {
  base: {
    borderBottomWidth: '1px',
    borderColor: 'border.default',
    w: 'full',
  },
})
