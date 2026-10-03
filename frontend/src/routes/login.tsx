import { createFileRoute, Link } from '@tanstack/react-router'

export const Route = createFileRoute('/login')({ component: LoginPage })

function LoginPage() {
  return (
    <section className="mx-auto max-w-2xl px-6 py-20">
      <span className="badge badge-outline mb-5">Coming soon</span>
      <h1 className="text-3xl font-bold">Account access</h1>
      <p className="mt-5 leading-relaxed text-base-content/75">
        This is where you’ll create an account and sign in with a passkey.
        Registration and login aren’t available yet.
      </p>
      <div className="mt-8 flex flex-wrap gap-3">
        <Link to="/" className="btn btn-primary">
          Back to home
        </Link>
        <Link to="/connections" className="btn btn-outline">
          Preview the workspace
        </Link>
      </div>
    </section>
  )
}
