import { Link } from '@tanstack/react-router'
import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react'
import { Button } from './button'

const GuideContext = createContext<(() => void) | null>(null)

export function useDemoGuide() {
  const open = useContext(GuideContext)
  if (!open) throw new Error('Demo guide is unavailable.')
  return open
}

const steps = [
  {
    title: 'Start the lab',
    summary: 'One command starts the gateway and three SSH hosts.',
  },
  {
    title: 'Sign in with a passkey',
    summary: 'Create an account for your keys and saved connections.',
  },
  {
    title: 'Upload the demo key',
    summary: 'Give the gateway a credential for the lab hosts.',
  },
  {
    title: 'Import the connections',
    summary: 'Load all three destinations from the sample config.',
  },
  {
    title: 'Verify the host',
    summary: 'Check who you are connecting to before opening a shell.',
  },
  {
    title: 'Explore the hosts',
    summary: 'Connect through the bastion and see network isolation in action.',
  },
]

export function DemoGuideProvider({ children }: { children: ReactNode }) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [step, setStep] = useState(0)
  const [open, setOpen] = useState(false)
  useEffect(() => {
    if (open) dialog.current?.showModal()
  }, [open])
  const close = () => dialog.current?.close()
  return (
    <GuideContext.Provider value={() => setOpen(true)}>
      {children}
      {open && (
        <dialog
          ref={dialog}
          className="side-drawer side-drawer-left"
          aria-labelledby="demo-guide-title"
          onClose={() => setOpen(false)}
        >
          <div className="drawer-panel">
            <div className="drawer-heading">
              <div>
                <p className="mb-1 text-xs font-semibold uppercase tracking-widest text-primary">
                  Local SSH lab
                </p>
                <h2 id="demo-guide-title" className="text-2xl font-bold">
                  Demo guide
                </h2>
              </div>
              <Button
                type="button"
                className="btn-ghost btn-sm"
                onClick={close}
              >
                Close guide
              </Button>
            </div>
            <p className="mt-5 text-sm text-base-content/70">
              Follow along from any page. Your place in this guide is kept while
              you navigate.
            </p>
            <ol className="my-6 space-y-1" aria-label="Demo steps">
              {steps.map((item, index) => (
                <li key={item.title}>
                  <button
                    type="button"
                    aria-current={step === index ? 'step' : undefined}
                    aria-controls="demo-step-content"
                    onClick={() => setStep(index)}
                    className={`flex w-full items-center gap-3 rounded-box px-3 py-2 text-left text-sm hover:bg-base-200 focus-visible:outline-2 focus-visible:outline-primary ${step === index ? 'bg-base-200 font-semibold text-primary' : 'text-base-content/70'}`}
                  >
                    <span className="flex size-6 shrink-0 items-center justify-center rounded-full border border-current text-xs">
                      {index + 1}
                    </span>
                    {item.title}
                  </button>
                </li>
              ))}
            </ol>
            <section
              id="demo-step-content"
              aria-labelledby="demo-step-title"
              className="mb-8 border-t border-base-300 pt-6"
            >
              <p className="text-xs font-semibold uppercase tracking-widest text-primary">
                Step {step + 1} of {steps.length}
              </p>
              <h3 id="demo-step-title" className="mt-2 text-xl font-semibold">
                {steps[step]?.title}
              </h3>
              <p className="mt-2 text-base-content/75">
                {steps[step]?.summary}
              </p>
              <div className="mt-5 space-y-4 text-sm leading-relaxed">
                {step === 0 && (
                  <>
                    <p>
                      From the repository root, with Docker Compose and Make
                      installed:
                    </p>
                    <Command>make demo</Command>
                    <p>
                      Open{' '}
                      <a className="link" href="http://localhost:8080">
                        localhost:8080
                      </a>
                      . The command initializes storage, applies migrations, and
                      waits for the demo to be ready.
                    </p>
                    <details className="rounded-box border border-base-300 p-3">
                      <summary className="cursor-pointer font-medium">
                        Using development mode?
                      </summary>
                      <div className="mt-3 space-y-3">
                        <Command>{'make migrate\nmake up'}</Command>
                        <p>
                          Use{' '}
                          <a className="link" href="http://localhost:5173">
                            localhost:5173
                          </a>{' '}
                          instead. Stop one mode before starting the other; both
                          share stored data.
                        </p>
                      </div>
                    </details>
                    <p>Already viewing the running app? Continue to sign-in.</p>
                  </>
                )}
                {step === 1 && (
                  <>
                    <p>
                      Choose a display name and create an account using your
                      browser’s passkey prompt, or sign in with an existing
                      passkey.
                    </p>
                    <p>
                      Your account keeps its own SSH keys, saved hosts, and
                      fingerprint approvals. Keep access to your passkey;
                      account recovery is not available.
                    </p>
                    <Link
                      to="/login"
                      onClick={close}
                      className="btn btn-outline btn-sm"
                    >
                      Sign in or register
                    </Link>
                  </>
                )}
                {step === 2 && (
                  <>
                    <p>
                      In SSH Keys, choose <strong>Upload SSH key</strong>. Name
                      it “Demo key” and select this file from your checkout:
                    </p>
                    <Command>demo/keys/demo_ed25519</Command>
                    <p>
                      Choose the private-key file, not the <code>.pub</code>{' '}
                      file. This intentionally public key is only for the
                      disposable lab. Never use it on a real host.
                    </p>
                    <Link
                      to="/keys"
                      onClick={close}
                      className="btn btn-outline btn-sm"
                    >
                      Open SSH Keys
                    </Link>
                  </>
                )}
                {step === 3 && (
                  <>
                    <p>
                      In Connections, choose <strong>Import SSH config</strong>{' '}
                      and select:
                    </p>
                    <Command>demo/ssh_config</Command>
                    <p>
                      Keep <code>bastion</code>, <code>target-1</code>, and{' '}
                      <code>target-2</code> selected. Map each identity to your
                      uploaded demo key. Both targets use the bastion as their
                      jump.
                    </p>
                    <p>
                      Choose <strong>Check selection</strong>, review the
                      settings, then <strong>Confirm import</strong>. Existing
                      names are not overwritten. If you already imported the
                      lab, use those saved connections.
                    </p>
                    <Link
                      to="/connections"
                      onClick={close}
                      className="btn btn-outline btn-sm"
                    >
                      Open Connections
                    </Link>
                  </>
                )}
                {step === 4 && (
                  <>
                    <p>
                      In Workspace, choose <strong>Connect</strong> and select a
                      host. Compare its displayed fingerprint with the
                      corresponding host’s output from this command, run in your
                      local repository terminal:
                    </p>
                    <Command>{`for host in bastion target-1 target-2; do
  echo "$host"
  docker compose exec "$host" sh -c 'for key in /var/lib/ssh-host-keys/*_key.pub; do ssh-keygen -lf "$key"; done'
done`}</Command>
                    <p>
                      Choose <strong>Approve fingerprint</strong> only after it
                      matches, then <strong>Open terminal</strong>. A target
                      connection checks both the bastion and the target.
                    </p>
                    <p>
                      A changed fingerprint blocks connection. Investigate the
                      change before resetting trust and approving a replacement.
                    </p>
                    <Link
                      to="/workspace"
                      onClick={close}
                      className="btn btn-outline btn-sm"
                    >
                      Open Workspace
                    </Link>
                  </>
                )}
                {step === 5 && (
                  <>
                    <p>Run this inside each browser terminal:</p>
                    <Command>cat /host-info.txt</Command>
                    <ul className="list-inside list-disc space-y-2">
                      <li>
                        <code>bastion</code> prints{' '}
                        <code>Bastion host: bastion</code>.
                      </li>
                      <li>
                        <code>target-1</code> prints{' '}
                        <code>Private target: target-1</code>.
                      </li>
                      <li>
                        <code>target-2</code> prints{' '}
                        <code>Private target: target-2</code>.
                      </li>
                    </ul>
                    <p>
                      Close one terminal before opening another. The targets are
                      on a private network that the app cannot reach directly;
                      SSH reaches them through the bastion.
                    </p>
                    <p>
                      You can visit Connections or SSH Keys without ending the
                      shell. Refresh, sign-out, or session expiry ends it.
                      Reconnect opens a fresh shell; terminal tabs and automatic
                      recovery are still planned.
                    </p>
                    <Link
                      to="/workspace"
                      onClick={close}
                      className="btn btn-outline btn-sm"
                    >
                      Open Workspace
                    </Link>
                  </>
                )}
              </div>
            </section>
            <div className="drawer-actions">
              <Button
                type="button"
                className="btn-outline"
                disabled={step === 0}
                onClick={() => setStep(step - 1)}
              >
                Previous step
              </Button>
              {step < steps.length - 1 ? (
                <Button
                  type="button"
                  className="btn-outline"
                  onClick={() => setStep(step + 1)}
                >
                  Next step
                </Button>
              ) : (
                <Button type="button" className="btn-outline" onClick={close}>
                  Finish guide
                </Button>
              )}
            </div>
          </div>
        </dialog>
      )}
    </GuideContext.Provider>
  )
}

function Command({ children }: { children: string }) {
  return (
    <pre className="overflow-x-auto rounded-box border border-base-300 bg-base-200 p-4 text-xs leading-relaxed">
      <code>{children}</code>
    </pre>
  )
}
