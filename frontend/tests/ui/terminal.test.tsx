import { afterEach, beforeEach, expect, test } from 'bun:test'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { TerminalPanel } from '~/routes/_authed/connections/-components/terminal-panel'
import {
  FakeObserver,
  FakeSocket,
  FakeTerminal,
  installTerminals,
  restoreTerminals,
} from './terminal-fakes'

const connection = {
  id: 'connection-id',
  name: 'Lab',
  host: 'bastion',
  port: 22,
  username: 'demo',
  ssh_key_id: 'key',
  created_at: '',
  updated_at: '',
}
beforeEach(installTerminals)
afterEach(() => {
  cleanup()
  restoreTerminals()
})
async function open() {
  const view = render(
    <StrictMode>
      <TerminalPanel connection={connection} onClose={() => {}} />
    </StrictMode>,
  )
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1))
  const socket = FakeSocket.instances[0]
  const terminal = FakeTerminal.instances[0]
  if (!socket || !terminal) throw new Error('Terminal was not initialized')
  return { view, socket, terminal }
}
test('terminal waits for shell readiness, forwards bytes and fits the viewport', async () => {
  const { view, socket, terminal } = await open()
  expect(socket.url).toBe(
    'ws://localhost:5173/api/connections/connection-id/terminal',
  )
  expect(socket.requestedProtocol).toBe('ssh-terminal.v1')
  expect(socket.binaryType).toBe('arraybuffer')
  act(() => {
    socket.open()
    terminal.data?.('ignored')
  })
  expect(socket.sent).toHaveLength(0)
  act(() => socket.status('connected'))
  await screen.findByText('Connected')
  expect(terminal.options.disableStdin).toBe(false)
  expect(socket.sent[0]).toBe('{"type":"resize","cols":100,"rows":30}')
  act(() => {
    terminal.data?.('猫\r\x03')
    terminal.binary?.('\xff\x00')
    socket.message(new Uint8Array([0xff, 0xe2, 0x82, 0xac]).buffer)
  })
  expect(socket.sent[1]).toEqual(new TextEncoder().encode('猫\r\x03'))
  expect(socket.sent[2]).toEqual(new Uint8Array([255, 0]))
  expect(terminal.writes[0]).toEqual(new Uint8Array([255, 226, 130, 172]))
  act(() => terminal.data?.('x'.repeat(32769)))
  expect((socket.sent[3] as Uint8Array).length).toBe(32768)
  expect((socket.sent[4] as Uint8Array).length).toBe(1)
  act(() => FakeObserver.instances[0]?.callback())
  expect(socket.sent.at(-1)).toBe('{"type":"resize","cols":100,"rows":30}')
  view.unmount()
  expect(terminal.disposed).toBe(true)
  expect(socket.closed).toBe(1)
  expect(FakeObserver.instances[0]?.disconnected).toBe(true)
  expect(terminal.data).toBeUndefined()
  expect(socket.onmessage).toBeNull()
})
test.each(['failed', 'disconnected'])(
  'final %s state is preserved and never reconnects',
  async (state) => {
    const { socket, terminal } = await open()
    act(() => {
      socket.open()
      socket.status('connected')
    })
    const late = socket.onmessage
    act(() => socket.status(state, 'Host ended the connection.'))
    await screen.findByText(/Host ended the connection/)
    expect(socket.closed).toBe(1)
    expect(terminal.options.disableStdin).toBe(true)
    expect(terminal.disposed).toBe(false)
    act(() =>
      late?.({ data: '{"type":"status","state":"connected"}' } as MessageEvent),
    )
    expect(screen.queryByText('Connected')).toBeNull()
    expect(FakeSocket.instances).toHaveLength(1)
  },
)
test('handshake failure and malformed status have useful failure states', async () => {
  const { socket } = await open()
  act(() => socket.onerror?.())
  await screen.findByRole('alert')
  expect(socket.onclose).toBeNull()
})
test('malformed controls and oversized output close the transport', async () => {
  const { socket } = await open()
  act(() => {
    socket.open()
    socket.status('connected')
    socket.message(new ArrayBuffer(32769))
  })
  await screen.findByText(/Terminal output exceeded its limit/)
  expect(socket.closed).toBe(1)
})
test('invalid status and a congested input queue close safely', async () => {
  const first = await open()
  act(() => first.socket.message('{'))
  await screen.findByText(/invalid terminal message/)
  first.view.unmount()
  installTerminals()
  const { socket, terminal } = await open()
  act(() => {
    socket.open()
    socket.status('connected')
    socket.bufferedAmount = 1024 * 1024
    terminal.data?.('x')
  })
  await screen.findByText(/input could not keep up/)
})
test.each([false, true])(
  'page departure disposes terminal, already ended: %s',
  async (ended) => {
    const { socket, terminal } = await open()
    if (ended) act(() => socket.status('disconnected', 'Shell exited.'))
    act(() => window.dispatchEvent(new Event('pagehide')))
    expect(socket.closed).toBe(1)
    expect(terminal.disposed).toBe(true)
  },
)

test('unmount during module loading cannot open a late socket', async () => {
  const view = render(
    <TerminalPanel connection={connection} onClose={() => {}} />,
  )
  view.unmount()
  await act(async () => {
    await import('~/routes/_authed/connections/-components/terminal-runtime')
  })
  expect(FakeSocket.instances).toHaveLength(0)
})
