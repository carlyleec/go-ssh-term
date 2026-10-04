import { createFileRoute, Link, redirect } from '@tanstack/react-router'
import queries from '~/api/queries'
import { AccessError, AccessPending } from '~/components/access-status'
import { useDemoGuide } from '~/components/demo-guide'
import { DemoNetwork } from './-components/demo-network'

export const Route = createFileRoute('/')({
  beforeLoad: async ({ context }) => {
    const account = await context.queryClient.fetchQuery(
      queries.auth.currentUser,
    )
    if (account) throw redirect({ to: '/workspace', replace: true })
  },
  pendingComponent: AccessPending,
  errorComponent: AccessError,
  component: LandingPage,
})

function LandingPage() {
  const openGuide = useDemoGuide()
  return (
    <div className="mx-auto max-w-6xl px-6 py-12 sm:py-16">
      <section className="grid items-start gap-8 lg:grid-cols-[1.5fr_1fr]">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-primary">
            A hands-on networking demo
          </p>
          <h1 className="mt-4 text-4xl leading-tight font-bold tracking-tight sm:text-5xl">
            Three hosts. One browser. Real SSH.
          </h1>
          <p className="mt-5 max-w-2xl text-lg leading-relaxed text-base-content/75">
            Sign in with a passkey, bring the lab’s SSH key, and open a shell.
            Start with the bastion, then reach two private hosts through a
            verified SSH jump.
          </p>
          <div className="mt-7 flex flex-wrap gap-3">
            <Link to="/login" className="btn btn-outline">
              Sign in or register
            </Link>
            <button
              type="button"
              className="btn btn-outline"
              onClick={openGuide}
            >
              Open demo guide
            </button>
          </div>
        </div>
        <div className="rounded-box border border-base-300 bg-base-200 p-6">
          <p className="text-sm font-semibold">Start from your checkout</p>
          <pre className="mt-4 overflow-x-auto rounded-box border border-base-300 bg-base-100 p-4 font-mono text-primary">
            <code>make demo</code>
          </pre>
          <p className="mt-4 text-sm leading-relaxed text-base-content/70">
            Requires Docker Compose, Make, and a passkey-capable browser. Open
            localhost:8080. No local Go or Bun installation needed.
          </p>
          <p className="mt-3 text-xs text-base-content/60">
            Already running the app? Start with sign-in.
          </p>
        </div>
      </section>
      <div className="mt-12">
        <DemoNetwork />
      </div>
      <section className="mt-12" aria-labelledby="walkthrough-title">
        <h2 id="walkthrough-title" className="text-2xl font-bold">
          Try the full path
        </h2>
        <ol className="mt-6 grid gap-6 md:grid-cols-3">
          <li className="border-t border-primary/50 pt-4">
            <p className="font-mono text-xs text-primary">01 / PREPARE</p>
            <h3 className="mt-2 font-semibold">Your account and key</h3>
            <p className="mt-2 text-sm leading-relaxed text-base-content/70">
              Register with a passkey. In SSH Keys, upload{' '}
              <code className="break-all">demo/keys/demo_ed25519</code> from the
              repository.
            </p>
          </li>
          <li className="border-t border-primary/50 pt-4">
            <p className="font-mono text-xs text-primary">02 / CONFIGURE</p>
            <h3 className="mt-2 font-semibold">Three saved destinations</h3>
            <p className="mt-2 text-sm leading-relaxed text-base-content/70">
              Import <code>demo/ssh_config</code> in Connections, map the demo
              key, and confirm. The private targets are configured to jump
              through the bastion.
            </p>
          </li>
          <li className="border-t border-primary/50 pt-4">
            <p className="font-mono text-xs text-primary">03 / CONNECT</p>
            <h3 className="mt-2 font-semibold">Verify, then explore</h3>
            <p className="mt-2 text-sm leading-relaxed text-base-content/70">
              In Workspace, compare host fingerprints and open a terminal. Run{' '}
              <code>cat /host-info.txt</code> on each host to see where you
              landed.
            </p>
          </li>
        </ol>
        <p className="mt-8 text-sm text-base-content/65">
          One terminal at a time for now. Close it before opening the next;
          terminal tabs and automatic recovery are still planned. The supplied
          SSH key is public and only for this disposable lab.
        </p>
      </section>
    </div>
  )
}
