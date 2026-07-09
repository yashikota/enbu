import type { ComponentProps } from 'react'
import { styled } from 'styled-system/jsx'

export type GroupProps = ComponentProps<typeof Group>
export const Group = styled('div', {
  base: {
    display: 'flex',
    alignItems: 'center',
  },
})
