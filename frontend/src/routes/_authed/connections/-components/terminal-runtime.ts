import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { z } from 'zod'
import { ENDPOINTS } from '~/api/endpoints'

const statusSchema = z
  .object({
    type: z.literal('status'),
    state: z.enum(['connecting', 'connected', 'failed', 'disconnected']),
    message: z.string().optional(),
  })
  .strict()
export type TerminalState = {
  state: 'connecting' | 'connected' | 'failed' | 'disconnected'
  message?: string
}
const dataLimit = 32 * 1024
const queueLimit = 1024 * 1024

export function mountTerminal(
  container: HTMLElement,
  id: string,
  onStatus: (status: TerminalState) => void,
): () => void {
  const terminal = new Terminal({
    cursorBlink: true,
    disableStdin: true,
    screenReaderMode: true,
    scrollback: 1000,
    theme: { background: '#101418', foreground: '#e5e7eb' },
  })
  const fit = new FitAddon()
  let socket: WebSocket | undefined
  let observer: ResizeObserver | undefined
  let disposed = false
  let ended = false
  let connected = false
  let pendingOutput = 0
  const subscriptions: { dispose: () => void }[] = []
  const encoder = new TextEncoder()
  function stopNetwork() {
    observer?.disconnect()
    for (const subscription of subscriptions.splice(0)) subscription.dispose()
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      socket.onclose = null
      if (socket.readyState < 2) socket.close(1000, 'terminal closed')
    }
    terminal.options.disableStdin = true
  }
  function finish(state: 'failed' | 'disconnected', message: string) {
    if (ended || disposed) return
    ended = true
    stopNetwork()
    onStatus({ state, message })
  }
  function dispose() {
    if (disposed) return
    disposed = true
    window.removeEventListener('pagehide', leave)
    stopNetwork()
    terminal.dispose()
    container.replaceChildren()
  }
  function leave() {
    finish('disconnected', 'Page closed.')
    dispose()
  }
  function send(data: Uint8Array<ArrayBuffer> | string) {
    if (
      ended ||
      disposed ||
      !connected ||
      socket?.readyState !== WebSocket.OPEN
    )
      return
    const size =
      typeof data === 'string'
        ? encoder.encode(data).byteLength
        : data.byteLength
    if (socket.bufferedAmount + size > queueLimit) {
      finish('disconnected', 'Terminal input could not keep up.')
      return
    }
    try {
      socket.send(data)
    } catch {
      finish('disconnected', 'Terminal connection lost.')
    }
  }
  function input(bytes: Uint8Array<ArrayBuffer>) {
    for (let offset = 0; offset < bytes.length && !ended; offset += dataLimit)
      send(bytes.subarray(offset, offset + dataLimit))
  }
  function resize() {
    if (ended || disposed) return
    const dimensions = fit.proposeDimensions()
    if (!dimensions) return
    const cols = Math.max(1, Math.min(1000, dimensions.cols))
    const rows = Math.max(1, Math.min(1000, dimensions.rows))
    terminal.resize(cols, rows)
    send(JSON.stringify({ type: 'resize', cols, rows }))
  }
  try {
    terminal.loadAddon(fit)
    terminal.open(container)
    const url = new URL(
      `${ENDPOINTS.connections}/${encodeURIComponent(id)}/terminal`,
      window.location.href,
    )
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    socket = new WebSocket(url.toString(), 'ssh-terminal.v1')
    socket.binaryType = 'arraybuffer'
    subscriptions.push(terminal.onData((data) => input(encoder.encode(data))))
    subscriptions.push(
      terminal.onBinary((data) =>
        input(Uint8Array.from(data, (char) => char.charCodeAt(0) & 255)),
      ),
    )
    observer = new ResizeObserver(resize)
    observer.observe(container)
    window.addEventListener('pagehide', leave)
    socket.onopen = () => {
      if (socket?.protocol !== 'ssh-terminal.v1')
        finish(
          'failed',
          'The server selected an unsupported terminal protocol.',
        )
    }
    socket.onmessage = (event: MessageEvent<unknown>) => {
      if (ended || disposed) return
      if (event.data instanceof ArrayBuffer) {
        if (
          !connected ||
          event.data.byteLength > dataLimit ||
          pendingOutput + event.data.byteLength > queueLimit
        ) {
          finish('disconnected', 'Terminal output exceeded its limit.')
          return
        }
        const size = event.data.byteLength
        pendingOutput += size
        terminal.write(new Uint8Array(event.data), () => {
          pendingOutput -= size
        })
        return
      }
      try {
        if (
          typeof event.data !== 'string' ||
          encoder.encode(event.data).byteLength > 1024
        )
          throw new Error('invalid status')
        const status = statusSchema.parse(JSON.parse(event.data))
        if (status.message && encoder.encode(status.message).byteLength > 256)
          throw new Error('invalid message')
        if (status.state === 'failed' || status.state === 'disconnected') {
          finish(status.state, status.message ?? 'SSH connection ended.')
          return
        }
        if (status.state === 'connected') {
          connected = true
          terminal.options.disableStdin = false
          resize()
          terminal.focus()
        }
        if (!ended) onStatus({ state: status.state, message: status.message })
      } catch {
        finish(
          connected ? 'disconnected' : 'failed',
          'The server sent an invalid terminal message.',
        )
      }
    }
    socket.onerror = () =>
      finish(
        connected ? 'disconnected' : 'failed',
        'Could not connect to the terminal. Check your session and try again.',
      )
    socket.onclose = () =>
      finish(
        connected ? 'disconnected' : 'failed',
        'Terminal connection closed.',
      )
    resize()
  } catch {
    finish(
      'failed',
      'Could not initialize the terminal. Close it and try again.',
    )
    dispose()
  }
  return dispose
}
