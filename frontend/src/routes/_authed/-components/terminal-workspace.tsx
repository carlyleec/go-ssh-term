import { useQueryClient } from '@tanstack/react-query'
import { useLocation } from '@tanstack/react-router'
import {
  createContext,
  type ReactNode,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react'
import { ApiError } from '~/api/apiClient'
import queries, { type SavedConnection } from '~/api/queries'
import { useAuth } from '~/hooks/use-auth'
import { HostDrawer } from '../workspace/-components/host-drawer'
import { TerminalPanel } from '../workspace/-components/terminal-panel'

const WorkspaceContext = createContext<{
  hasTerminal: boolean
  inspect: (connection: SavedConnection) => void
} | null>(null)

export function useTerminalWorkspace() {
  const workspace = useContext(WorkspaceContext)
  if (!workspace) throw new Error('Terminal workspace is unavailable.')
  return workspace
}

export function TerminalWorkspace({ children }: { children: ReactNode }) {
  const { account, isSigningOut } = useAuth()
  const client = useQueryClient()
  const pathname = useLocation({ select: (location) => location.pathname })
  const visible = pathname.replace(/\/$/, '') === '/workspace'
  const [terminal, setTerminal] = useState<SavedConnection | null>(null)
  const [hostTarget, setHostTarget] = useState<SavedConnection | null>(null)
  const [attempt, setAttempt] = useState(0)
  const [notice, setNotice] = useState('')
  const [reconnectPending, setReconnectPending] = useState(false)
  const requestID = useRef(0)
  const reconnectActive = useRef(false)
  const mounted = useRef(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      requestID.current++
    }
  }, [])
  useEffect(() => {
    if (!visible) {
      requestID.current++
      reconnectActive.current = false
      setReconnectPending(false)
      setHostTarget(null)
    }
  }, [visible])

  async function reconnect() {
    if (!terminal || !account || reconnectActive.current || isSigningOut) return
    reconnectActive.current = true
    const request = ++requestID.current
    setReconnectPending(true)
    setNotice('')
    try {
      const latest = await client.fetchQuery({
        ...queries.connections.options(account.id),
        staleTime: 0,
      })
      if (!mounted.current || request !== requestID.current) return
      const target = latest.find((item) => item.id === terminal.id)
      if (!target) {
        setNotice(
          'This saved connection was deleted. Close the terminal and choose another connection.',
        )
        return
      }
      setHostTarget(target)
    } catch (error) {
      if (!mounted.current || request !== requestID.current) return
      if (error instanceof ApiError && error.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
      else setNotice('Could not prepare reconnect. Try again.')
    } finally {
      if (mounted.current && request === requestID.current) {
        reconnectActive.current = false
        setReconnectPending(false)
      }
    }
  }

  return (
    <WorkspaceContext.Provider
      value={{ hasTerminal: terminal !== null, inspect: setHostTarget }}
    >
      {children}
      <div hidden={!visible} className="mx-auto w-full max-w-6xl px-6 pb-12">
        {notice && <p role="status">{notice}</p>}
        {terminal && (
          <TerminalPanel
            key={attempt}
            connection={terminal}
            onReconnect={() => void reconnect()}
            reconnectPending={
              reconnectPending || isSigningOut || hostTarget !== null
            }
            onClose={() => {
              requestID.current++
              reconnectActive.current = false
              setReconnectPending(false)
              setHostTarget(null)
              setTerminal(null)
              setNotice('')
            }}
          />
        )}
      </div>
      {visible && hostTarget && account && (
        <HostDrawer
          accountID={account.id}
          connection={hostTarget}
          onClose={() => setHostTarget(null)}
          onConnect={() => {
            if (!mounted.current || isSigningOut) return
            setAttempt((value) => value + 1)
            setTerminal(hostTarget)
            setHostTarget(null)
            setNotice('')
          }}
        />
      )}
    </WorkspaceContext.Provider>
  )
}
