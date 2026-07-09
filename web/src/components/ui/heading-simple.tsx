import { styled } from 'styled-system/jsx'

export const Heading = styled('h2', {
  base: {
    fontWeight: 'semibold',
    lineHeight: 'tight',
  },
  variants: {
    size: {
      xs: { fontSize: 'xs' },
      sm: { fontSize: 'sm' },
      md: { fontSize: 'md' },
      lg: { fontSize: 'lg' },
      xl: { fontSize: 'xl' },
      '2xl': { fontSize: '2xl' },
      '3xl': { fontSize: '3xl' },
      '4xl': { fontSize: '4xl' },
    },
  },
})

export type HeadingProps = React.ComponentProps<typeof Heading>
