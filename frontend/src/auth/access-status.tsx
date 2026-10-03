import { Link, useRouter } from '@tanstack/react-router'

export function AccessPending() {
  return (
    <p role="status" className="mx-auto max-w-2xl px-6 py-20">
      Checking your session…
    </p>
  )
}

export function AccessError() {
  const router = useRouter()
  return (
    <section className="mx-auto max-w-2xl px-6 py-20">
      <h1 className="text-3xl font-bold">Could not check your session</h1>
      <p role="alert" className="mt-4">
        Check your connection and try again. The server may be temporarily
        unavailable.
      </p>
      <button
        type="button"
        className="btn btn-primary mt-6"
        onClick={() => {
          void router.invalidate()
        }}
      >
        Try again
      </button>
      <Link to="/" className="btn btn-ghost mt-6 ml-3">
        Back to home
      </Link>
    </section>
  )
}
