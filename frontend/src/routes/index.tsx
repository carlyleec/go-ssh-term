import { createFileRoute, Link } from '@tanstack/react-router'

export const Route = createFileRoute('/')({ component: LandingPage })

function LandingPage() {
  return (
    <div className="mx-auto max-w-6xl px-6 py-16 sm:py-24">
      <section className="max-w-3xl">
        <span className="badge badge-outline mb-6">In development</span>
        <h1 className="text-4xl leading-tight font-bold tracking-tight sm:text-6xl">
          Your SSH workspace, in the browser.
        </h1>
        <p className="mt-6 max-w-2xl text-lg leading-relaxed text-base-content/75">
          A local browser SSH gateway for exploring Go networking,
          authentication, and connection management. The planned demo connects a
          bastion and two private hosts through a single workspace.
        </p>
        <div className="mt-8 flex flex-wrap gap-3">
          <Link to="/login" className="btn btn-primary">
            Go to account access
          </Link>
          <Link to="/connections" className="btn btn-outline">
            Preview the workspace
          </Link>
        </div>
        <p className="mt-4 text-sm text-base-content/65">
          Authentication and SSH connections aren’t available yet.
        </p>
      </section>

      <section aria-labelledby="demo-heading" className="mt-20">
        <h2 id="demo-heading" className="text-2xl font-bold">
          What the finished demo will let you try
        </h2>
        <ol className="mt-6 grid gap-6 md:grid-cols-3">
          <li className="border-t-2 border-primary pt-5">
            <h3 className="text-lg font-semibold">1. Create an account</h3>
            <p className="mt-2 leading-relaxed text-base-content/75">
              Register and sign in with a passkey to access your own workspace.
            </p>
          </li>
          <li className="border-t-2 border-primary pt-5">
            <h3 className="text-lg font-semibold">2. Set up the lab</h3>
            <p className="mt-2 leading-relaxed text-base-content/75">
              Upload the demo SSH key and import the supplied host
              configurations.
            </p>
          </li>
          <li className="border-t-2 border-primary pt-5">
            <h3 className="text-lg font-semibold">3. Open a terminal</h3>
            <p className="mt-2 leading-relaxed text-base-content/75">
              Approve host fingerprints, connect through the bastion, and switch
              between terminal tabs.
            </p>
          </li>
        </ol>
      </section>
    </div>
  )
}
