import { createRootRoute } from '@tanstack/react-router'
import { App, NotFoundPage } from '../app'

export const Route = createRootRoute({
  component: App,
  notFoundComponent: NotFoundPage,
})
