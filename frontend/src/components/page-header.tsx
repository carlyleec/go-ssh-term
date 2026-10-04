import type { ReactNode } from 'react'

export function PageHeader({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-4">
      <div className="min-w-0">
        <h1 className="text-3xl font-bold">{title}</h1>
        <p className="mt-2 text-base-content/75">{description}</p>
      </div>
      <div className="ml-auto flex flex-wrap justify-end gap-3">{children}</div>
    </div>
  )
}
