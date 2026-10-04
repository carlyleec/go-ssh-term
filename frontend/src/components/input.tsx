import type { ComponentProps } from 'react'

export function Input({
  className = '',
  type,
  ...props
}: ComponentProps<'input'>) {
  return (
    <input
      type={type}
      className={`${type === 'file' ? 'file-input' : 'input'} ${className}`}
      {...props}
    />
  )
}
