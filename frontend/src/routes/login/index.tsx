import { browserSupportsWebAuthn } from '@simplewebauthn/browser'
import { useForm } from '@tanstack/react-form'
import type { QueryClient } from '@tanstack/react-query'
import { useMutation } from '@tanstack/react-query'
import {
  createFileRoute,
  Link,
  redirect,
  useNavigate,
} from '@tanstack/react-router'
import { useRef } from 'react'
import { z } from 'zod'
import { ApiError } from '~/api/apiClient'
import queries, { type AccessAttempt } from '~/api/queries'
import { AccessError, AccessPending } from '~/components/access-status'
import { Button } from '~/components/button'
import { Input } from '~/components/input'

export const Route = createFileRoute('/login/')({
  beforeLoad: ({ context }) => redirectSignedIn(context.queryClient),
  pendingComponent: AccessPending,
  errorComponent: AccessError,
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const { queryClient } = Route.useRouteContext()
  const activeAttempt = useRef(false)
  const supported = window.isSecureContext && browserSupportsWebAuthn()
  const access = useMutation(queries.auth.access)

  async function submit(attempt: AccessAttempt) {
    if (activeAttempt.current || !supported) return
    activeAttempt.current = true
    try {
      await access.mutateAsync(attempt)
    } catch {
      // The mutation error is rendered below for either account-access action.
      return
    } finally {
      activeAttempt.current = false
    }
    await queryClient.cancelQueries({
      queryKey: queries.auth.currentUser.queryKey,
    })
    queryClient.removeQueries({
      queryKey: queries.auth.currentUser.queryKey,
    })
    await navigate({ to: '/workspace', replace: true })
  }

  const form = useForm({
    defaultValues: { displayName: '' },
    validators: { onChange: registrationSchema },
    onSubmit: async ({ value }) => {
      await submit({ kind: 'register', displayName: value.displayName })
    },
  })
  const disabled = !supported || access.isPending || access.isSuccess

  return (
    <section className="mx-auto max-w-lg px-6 py-16">
      <h1 className="text-3xl font-bold">Account access</h1>
      <p className="mt-4 text-base-content/75">
        Use a passkey to enter your SSH workspace. No password needed.
      </p>
      {!supported && (
        <p role="alert" className="alert alert-warning mt-6">
          Passkeys require a supported browser and a secure address. Open the
          documented localhost URL or use HTTPS.
        </p>
      )}
      {access.error && (
        <p role="alert" className="alert alert-error mt-6">
          {accessErrorMessage(access.error)}
        </p>
      )}
      <p role="status" className="mt-4 text-sm" aria-live="polite">
        {access.isPending
          ? 'Follow your browser’s passkey prompt. This may take a moment.'
          : access.isSuccess
            ? 'Signed in. Opening your workspace…'
            : ''}
      </p>
      {access.isSuccess && (
        <Link to="/workspace" className="link">
          Open workspace
        </Link>
      )}
      <div className="mt-6 rounded-box border border-base-300 bg-base-200 p-6">
        <h2 className="text-xl font-semibold">Welcome back</h2>
        <p className="mt-2 text-sm text-base-content/75">
          Choose the passkey you used to create your account.
        </p>
        <Button
          type="button"
          className="btn-primary mt-5 w-full"
          disabled={disabled}
          onClick={() => {
            void submit({ kind: 'login' })
          }}
        >
          {access.isPending && access.variables?.kind === 'login'
            ? 'Signing in…'
            : 'Sign in with a passkey'}
        </Button>
      </div>
      <form
        className="mt-6 rounded-box border border-base-300 p-6"
        onSubmit={(event) => {
          event.preventDefault()
          event.stopPropagation()
          void form.handleSubmit()
        }}
      >
        <h2 className="text-xl font-semibold">Create an account</h2>
        <p className="mt-2 text-sm text-base-content/75">
          Choose a display name, then save a passkey on your device or in your
          password manager.
        </p>
        <form.Field name="displayName">
          {(field) => (
            <div className="mt-5">
              <label htmlFor={field.name} className="mb-2 block font-medium">
                Display name
              </label>
              <Input
                id={field.name}
                name={field.name}
                className="w-full"
                autoComplete="nickname"
                value={field.state.value}
                disabled={disabled}
                onBlur={field.handleBlur}
                onChange={(event) => field.handleChange(event.target.value)}
                aria-invalid={field.state.meta.errors.length > 0}
                aria-describedby="display-name-help display-name-error"
              />
              <p
                id="display-name-help"
                className="mt-2 text-sm text-base-content/65"
              >
                A label for your account, not a unique username.
              </p>
              <p
                id="display-name-error"
                className="mt-2 text-sm text-error"
                role="alert"
              >
                {[
                  ...new Set(
                    field.state.meta.errors.map((error) => error?.message),
                  ),
                ].join(' ')}
              </p>
            </div>
          )}
        </form.Field>
        <p className="mt-4 text-sm text-base-content/75">
          Each account has one passkey. Account recovery is unavailable, so keep
          access to your passkey.
        </p>
        <form.Subscribe selector={(state) => state.isSubmitting}>
          {(isSubmitting) => (
            <Button
              type="submit"
              className="btn-outline mt-5 w-full"
              disabled={disabled || isSubmitting}
            >
              {access.isPending && access.variables?.kind === 'register'
                ? 'Creating account…'
                : 'Create account with a passkey'}
            </Button>
          )}
        </form.Subscribe>
      </form>
    </section>
  )
}

export async function redirectSignedIn(queryClient: QueryClient) {
  const account = await queryClient.fetchQuery(queries.auth.currentUser)
  if (account) throw redirect({ to: '/workspace', replace: true })
}

export function accessErrorMessage(error: Error): string {
  if (error instanceof ApiError) {
    if (error.status === 0 || (error.status >= 200 && error.status < 300))
      return `${error.message} If you just created a passkey, try signing in first.`
    return error.message
  }
  const cause = error.cause instanceof Error ? error.cause : error
  if (cause.name === 'NotAllowedError' || cause.name === 'AbortError') {
    return 'The passkey prompt was cancelled, timed out, or was not allowed. Try again when you’re ready.'
  }
  if (cause.name === 'InvalidStateError')
    return 'This passkey may already be registered. Try signing in instead.'
  if (cause.name === 'SecurityError')
    return 'Passkeys are unavailable at this address. Use the documented localhost URL or HTTPS.'
  return 'Could not use your passkey. Check that your browser and device support passkeys, then try again.'
}

export const registrationSchema = z.object({
  displayName: z.string().refine(
    (value) => {
      const name = value.trim()
      return name.length > 0 && [...name].length <= 64 && !/\p{Cc}/u.test(name)
    },
    { error: 'Enter 1–64 characters without control characters.' },
  ),
})
