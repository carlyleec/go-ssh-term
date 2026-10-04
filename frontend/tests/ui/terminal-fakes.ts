import { mock } from 'bun:test'

export class FakeTerminal {
  static instances: FakeTerminal[] = []
  options = { disableStdin: true }
  disposed = false
  writes: Uint8Array[] = []
  sizes: [number, number][] = []
  data: ((value: string) => void) | undefined
  binary: ((value: string) => void) | undefined
  constructor() {
    FakeTerminal.instances.push(this)
  }
  loadAddon() {}
  open() {}
  focus() {}
  resize(cols: number, rows: number) {
    this.sizes.push([cols, rows])
  }
  onData(callback: (data: string) => void) {
    this.data = callback
    return {
      dispose: () => {
        this.data = undefined
      },
    }
  }
  onBinary(callback: (data: string) => void) {
    this.binary = callback
    return {
      dispose: () => {
        this.binary = undefined
      },
    }
  }
  write(data: Uint8Array, callback: () => void) {
    this.writes.push(data)
    callback()
  }
  dispose() {
    this.disposed = true
  }
}
class FakeFit {
  proposeDimensions() {
    return { cols: 100, rows: 30 }
  }
}
export class FakeSocket {
  static OPEN = 1
  static instances: FakeSocket[] = []
  readyState = 0
  bufferedAmount = 0
  binaryType = 'blob'
  protocol = 'ssh-terminal.v1'
  closed = 0
  sent: (string | Uint8Array)[] = []
  onopen: (() => void) | null = null
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  constructor(
    public url: string,
    public requestedProtocol: string,
  ) {
    FakeSocket.instances.push(this)
  }
  send(data: string | Uint8Array) {
    this.sent.push(data)
  }
  close() {
    this.readyState = 3
    this.closed++
  }
  open() {
    this.readyState = 1
    this.onopen?.()
  }
  message(data: unknown) {
    this.onmessage?.({ data } as MessageEvent<unknown>)
  }
  status(state: string, message?: string) {
    this.message(JSON.stringify({ type: 'status', state, message }))
  }
}
export class FakeObserver {
  static instances: FakeObserver[] = []
  disconnected = false
  constructor(public callback: () => void) {
    FakeObserver.instances.push(this)
  }
  observe() {}
  disconnect() {
    this.disconnected = true
  }
}
mock.module('@xterm/xterm', () => ({ Terminal: FakeTerminal }))
mock.module('@xterm/addon-fit', () => ({ FitAddon: FakeFit }))
const originalSocket = globalThis.WebSocket
const originalObserver = globalThis.ResizeObserver
export function installTerminals() {
  FakeTerminal.instances = []
  FakeSocket.instances = []
  FakeObserver.instances = []
  globalThis.WebSocket = FakeSocket as unknown as typeof WebSocket
  globalThis.ResizeObserver = FakeObserver as unknown as typeof ResizeObserver
}
export function restoreTerminals() {
  globalThis.WebSocket = originalSocket
  globalThis.ResizeObserver = originalObserver
}
