import { useId, useState } from 'react'

type NodeID =
  | 'browser'
  | 'frontend'
  | 'app'
  | 'bastion'
  | 'target-1'
  | 'target-2'
type Destination = 'bastion' | 'target-1' | 'target-2'

const nodeInfo: Record<
  NodeID,
  { title: string; networks: string; description: string }
> = {
  browser: {
    title: 'Your browser',
    networks: 'Outside Docker',
    description:
      'Passkeys authenticate your account. HTTP carries API requests; a WebSocket carries terminal input and output. The browser does not make SSH connections.',
  },
  frontend: {
    title: 'frontend · Vite',
    networks: 'default',
    description:
      'Development only. Vite serves React on localhost:5173 and proxies /api requests and terminal WebSockets to app:8080 on the default network.',
  },
  app: {
    title: 'app · SSH gateway',
    networks: 'default + lab-gateway',
    description:
      'Go authenticates the session, verifies host trust, and opens SSH connections. In demo mode it also serves React on localhost:8080. SQLite and the application encryption key live in separate persistent volumes.',
  },
  bastion: {
    title: 'bastion · OpenSSH',
    networks: 'lab-gateway + lab-private',
    description:
      'The jump host has an interface on both lab networks. The gateway can open a shell here, or use its SSH forwarding to reach a private target. SSH listens on port 22; no host port is published.',
  },
  'target-1': {
    title: 'target-1 · OpenSSH',
    networks: 'lab-private only',
    description:
      'A private host reached through the bastion. It uses demo on port 22 and the uploaded demo key. Its identifying file prints “Private target: target-1”. No host port is published.',
  },
  'target-2': {
    title: 'target-2 · OpenSSH',
    networks: 'lab-private only',
    description:
      'A second, independent private host reached through the bastion. It uses demo on port 22 and the uploaded demo key. Its identifying file prints “Private target: target-2”. No host port is published.',
  },
}

export function DemoNetwork() {
  const [development, setDevelopment] = useState(false)
  const [destination, setDestination] = useState<Destination>('target-1')
  const [selected, setSelected] = useState<NodeID>('target-1')
  const marker = useId().replace(/:/g, '')
  const info = nodeInfo[selected]
  const path = [
    'Browser',
    ...(development ? ['frontend'] : []),
    'app',
    'bastion',
    ...(destination === 'bastion' ? [] : [destination]),
  ]
  const nodes: {
    id: NodeID
    x: number
    y: number
    title: string
    subtitle: string
  }[] = [
    {
      id: 'browser',
      x: 20,
      y: 220,
      title: 'Browser',
      subtitle: development ? 'localhost:5173' : 'localhost:8080',
    },
    ...(development
      ? [
          {
            id: 'frontend' as const,
            x: 245,
            y: 115,
            title: 'frontend',
            subtitle: 'Vite · :5173',
          },
        ]
      : []),
    {
      id: 'app',
      x: 300,
      y: 220,
      title: 'app',
      subtitle: development ? 'Go + Air · :8080' : 'Go + React · :8080',
    },
    {
      id: 'bastion',
      x: 600,
      y: 220,
      title: 'bastion',
      subtitle: 'OpenSSH · :22',
    },
    {
      id: 'target-1',
      x: 880,
      y: 125,
      title: 'target-1',
      subtitle: 'OpenSSH · :22',
    },
    {
      id: 'target-2',
      x: 880,
      y: 310,
      title: 'target-2',
      subtitle: 'OpenSSH · :22',
    },
  ]
  return (
    <section
      aria-labelledby="network-title"
      className="overflow-hidden rounded-box border border-base-300 bg-base-200"
    >
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-base-300 p-6">
        <div>
          <p className="text-xs font-semibold uppercase tracking-widest text-primary">
            Explore the topology
          </p>
          <h2 id="network-title" className="mt-2 text-2xl font-bold">
            One gateway. Two network boundaries.
          </h2>
          <p className="mt-2 max-w-xl text-sm text-base-content/70">
            Select a container to inspect it. Select a destination to trace the
            SSH path.
          </p>
        </div>
        <fieldset className="join" aria-label="Compose mode">
          <button
            type="button"
            aria-pressed={!development}
            className={`btn btn-sm join-item ${!development ? 'btn-primary' : 'btn-outline'}`}
            onClick={() => {
              setDevelopment(false)
              if (selected === 'frontend') setSelected('app')
            }}
          >
            Demo
          </button>
          <button
            type="button"
            aria-pressed={development}
            className={`btn btn-sm join-item ${development ? 'btn-primary' : 'btn-outline'}`}
            onClick={() => setDevelopment(true)}
          >
            Development
          </button>
        </fieldset>
      </div>
      <div className="flex flex-wrap items-center gap-2 px-6 pt-5">
        <span className="mr-2 text-xs font-semibold uppercase tracking-wider text-base-content/60">
          Destination
        </span>
        {(['bastion', 'target-1', 'target-2'] as const).map((host) => (
          <button
            key={host}
            type="button"
            aria-label={`Trace ${host}`}
            aria-pressed={destination === host}
            onClick={() => {
              setDestination(host)
              setSelected(host)
            }}
            className={`btn btn-sm ${destination === host ? 'btn-primary' : 'btn-ghost'}`}
          >
            {host}
          </button>
        ))}
      </div>
      <p className="px-6 pt-3 text-xs text-base-content/60 sm:hidden">
        Scroll the diagram sideways to explore all hosts.
      </p>
      <section
        className="overflow-x-auto"
        aria-label="Interactive network diagram"
      >
        <div
          className="relative min-w-[900px]"
          style={{ aspectRatio: '1100 / 440' }}
        >
          <div
            className="network-zone border-info/40 bg-info/5"
            style={{
              left: '20%',
              top: '10.23%',
              width: '20.91%',
              height: '81.82%',
            }}
          >
            <span className="network-label text-info">default</span>
          </div>
          <div
            className="network-zone border-primary/40 bg-primary/5"
            style={{
              left: '35.45%',
              top: '18.18%',
              width: '29.55%',
              height: '73.86%',
            }}
          >
            <span
              className="network-label text-primary"
              style={{ left: '25%' }}
            >
              lab-gateway
            </span>
          </div>
          <div
            className="network-zone border-success/40 bg-success/5"
            style={{
              left: '62.73%',
              top: '10.23%',
              width: '35.45%',
              height: '81.82%',
            }}
          >
            <span
              className="network-label text-success"
              style={{ left: '28%' }}
            >
              lab-private · internal
            </span>
          </div>
          <svg
            aria-hidden="true"
            viewBox="0 0 1100 440"
            className="pointer-events-none absolute inset-0 size-full fill-none"
          >
            <defs>
              <marker
                id={marker}
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerWidth="6"
                markerHeight="6"
                orient="auto-start-reverse"
              >
                <path d="M0 0 10 5 0 10z" fill="currentColor" stroke="none" />
              </marker>
            </defs>
            <g
              className="text-primary"
              stroke="currentColor"
              strokeWidth="2.5"
              markerEnd={`url(#${marker})`}
            >
              {development ? (
                <>
                  <path d="M200 258 H220 V153 H245" />
                  <path d="M315 191 V220" />
                </>
              ) : (
                <path d="M200 258 H300" />
              )}
              <path d="M480 258 H600" />
            </g>
            <path
              d="M780 258 H830 V163 H880"
              className={
                destination === 'target-1'
                  ? 'text-primary'
                  : 'text-base-content/20'
              }
              stroke="currentColor"
              strokeWidth="2.5"
              markerEnd={`url(#${marker})`}
            />
            <path
              d="M780 258 H830 V348 H880"
              className={
                destination === 'target-2'
                  ? 'text-primary'
                  : 'text-base-content/20'
              }
              stroke="currentColor"
              strokeWidth="2.5"
              markerEnd={`url(#${marker})`}
            />
            <text x="520" y="243" className="fill-base-content/60 text-[12px]">
              SSH
            </text>
            <text x="28" y="330" className="fill-base-content/60 text-[12px]">
              HTTP + WebSocket
            </text>
          </svg>
          {nodes.map((node) => (
            <button
              key={node.id}
              type="button"
              aria-label={`Inspect ${node.title}`}
              aria-pressed={selected === node.id}
              onClick={() => {
                setSelected(node.id)
                if (
                  node.id === 'bastion' ||
                  node.id === 'target-1' ||
                  node.id === 'target-2'
                )
                  setDestination(node.id)
              }}
              className={`absolute flex flex-col items-start justify-center rounded-xl border bg-base-100 px-4 text-left shadow-md transition-colors focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary ${selected === node.id ? 'border-primary ring-2 ring-primary/20' : 'border-base-content/20 hover:border-primary/60'}`}
              style={{
                left: `${node.x / 11}%`,
                top: `${node.y / 4.4}%`,
                width: `${(node.id === 'frontend' ? 140 : 180) / 11}%`,
                height: `${76 / 4.4}%`,
              }}
            >
              <span className="font-mono text-sm font-semibold">
                {node.title}
              </span>
              <span className="mt-1 text-xs text-base-content/65">
                {node.subtitle}
              </span>
            </button>
          ))}
        </div>
      </section>
      <div className="grid gap-6 border-t border-base-300 bg-base-100 p-6 md:grid-cols-2">
        <div aria-live="polite" aria-atomic="true">
          <h3 className="font-semibold">{info.title}</h3>
          <p className="mt-1 font-mono text-xs text-primary">{info.networks}</p>
          <p className="mt-3 text-sm leading-relaxed text-base-content/75">
            {info.description}
          </p>
        </div>
        <div>
          <p className="text-xs font-semibold uppercase tracking-widest text-base-content/60">
            Selected traffic path
          </p>
          <p className="mt-2 break-words font-mono text-sm" aria-live="polite">
            {path.join(' → ')}
          </p>
          <p className="mt-3 text-sm leading-relaxed text-base-content/75">
            The app is not attached to lab-private. Both targets require a jump
            through the bastion; neither exposes an SSH port to your host.
          </p>
        </div>
      </div>
      <p className="border-t border-base-300 px-6 py-3 text-xs leading-relaxed text-base-content/60">
        Compose topology, not live connection status. Network names are shown
        without project prefixes; Docker assigns the subnet ranges. Setup and
        build helpers are omitted.
      </p>
    </section>
  )
}
