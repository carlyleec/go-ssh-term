import { browserSupportsWebAuthn } from '@simplewebauthn/browser'
import { useForm } from '@tanstack/react-form'
import { useMutation } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useRef } from 'react'
import {
  type AccessAttempt,
  accessErrorMessage,
  accessWithPasskey,
  validateDisplayName,
} from '../auth/passkeys'

export const Route = createFileRoute('/login')({ component: LoginPage })

function LoginPage() {
  const navigate = useNavigate()
  const activeAttempt = useRef(false)
  const supported = window.isSecureContext && browserSupportsWebAuthn()
  const access = useMutation({
    mutationFn: accessWithPasskey,
    retry: false,
    // An interactive prompt must not be queued for a later network reconnect.
    networkMode: 'always',
  })

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
    await navigate({ to: '/connections' })
  }

  const form = useForm({
    defaultValues: { displayName: '' },
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
        <Link to="/connections" className="link">
          Open workspace
        </Link>
      )}
      <div className="mt-6 rounded-box border border-base-300 bg-base-200 p-6">
        <h2 className="text-xl font-semibold">Welcome back</h2>
        <p className="mt-2 text-sm text-base-content/75">
          Choose the passkey you used to create your account.
        </p>
        <button
          type="button"
          className="btn btn-primary mt-5 w-full"
          disabled={disabled}
          onClick={() => {
            void submit({ kind: 'login' })
          }}
        >
          {access.isPending && access.variables?.kind === 'login'
            ? 'Signing in…'
            : 'Sign in with a passkey'}
        </button>
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
        <form.Field
          name="displayName"
          validators={{
            onBlur: ({ value }) => validateDisplayName(value),
            onSubmit: ({ value }) => validateDisplayName(value),
          }}
        >
          {(field) => (
            <div className="mt-5">
              <label htmlFor={field.name} className="mb-2 block font-medium">
                Display name
              </label>
              <input
                id={field.name}
                name={field.name}
                className="input w-full"
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
                {field.state.meta.errors.join(' ')}
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
            <button
              type="submit"
              className="btn btn-outline mt-5 w-full"
              disabled={disabled || isSubmitting}
            >
              {access.isPending && access.variables?.kind === 'register'
                ? 'Creating account…'
                : 'Create account with a passkey'}
            </button>
          )}
        </form.Subscribe>
      </form>
    </section>
  )
}
