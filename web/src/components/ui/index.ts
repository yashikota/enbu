export * from './badge'
export * from './button'
export * from './input'
export * from './loader'
export * from './separator'
export * from './spinner'
export * from './text'
export * from './textarea'

// Named namespace exports to avoid conflicts with Alert and Popover compound components
export * as Alert from './alert'
export * as Popover from './popover'

// Heading exported directly (simple component)
export { Heading } from './heading-simple'
