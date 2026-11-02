import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import './style.css'

type Session = {
  provider: 'codex' | 'claude'
  provider_version: string
  installation_id: string
  workspace_id: string
  session_id: string
  turn_id: string
  execution_state: 'unknown' | 'running' | 'completed' | 'failed' | 'interrupted'
  last_known_execution_state: string
  source_connected: boolean
  last_seen: string
}
type Fleet = { cursor: number; mode: 'demo' | 'observe'; sessions: Session[] }
type Connection = 'connecting' | 'live' | 'reconnecting'

const names = { codex: 'Codex', claude: 'Claude Code' }
const states = { running: 'Running', completed: 'Execution complete', failed: 'Failed', interrupted: 'Interrupted', unknown: 'Unknown' }

function identity(s: Session) { return `${s.provider}/${s.installation_id}/${s.workspace_id}/${s.session_id}` }
function ageLabel(value: string, now: number) {
  const seconds = Math.max(0, Math.floor((now - Date.parse(value)) / 1000))
  if (!Number.isFinite(seconds)) return 'Time unavailable'
  if (seconds < 2) return 'Just now'
  if (seconds < 60) return `${seconds}s ago`
  return `${Math.floor(seconds / 60)}m ago`
}

type ReviewReport = {
  task_id: string; review_id: string; revision: string; state: string; expires_at: string
  previews: {path: string; text: string; truncated: boolean}[]
  artifacts: { path: string; sha256: string }[]
  tests: { id: string; passed: boolean; output_sha256: string; output_bytes: number }[]
}
function ReviewPanel() {
  const [data, setData] = useState<{report: ReviewReport; csrf: string} | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [unavailable, setUnavailable] = useState(false)
  const load = async () => {
    const response = await fetch('/v0/review')
    if (response.status === 404) { setUnavailable(true); return }
    if (!response.ok) throw new Error('Sübutları yeniləmək mümkün olmadı.')
    setData(await response.json()); setUnavailable(false)
  }
  useEffect(() => {
    let active = true
    const refresh = () => { if (active) void load().catch(() => { if (active) setError('Əlaqə kəsildi. Təsdiqdən əvvəl səhifəni yeniləyin.') }) }
    refresh()
    const timer = window.setInterval(refresh, 10000)
    return () => { active = false; window.clearInterval(timer) }
  }, [])
  const approve = async () => {
    if (!data || busy) return
    setBusy(true); setError('')
    try {
      const response = await fetch('/v0/review/approve', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-AgentOS-CSRF': data.csrf }, body: JSON.stringify({review_id: data.report.review_id}) })
      if (!response.ok) throw new Error('Təsdiq alınmadı. Sübut dəyişmiş, vaxtı keçmiş və ya artıq istifadə edilmiş ola bilər.')
      const report = await response.json() as ReviewReport
      setData({...data, report})
    } catch (e) { setError(e instanceof Error ? e.message : 'Təsdiq alınmadı.'); await load().catch(() => {}) }
    finally { setBusy(false) }
  }
  const report = data?.report
  const pending = report?.state === 'awaiting_user_approval'
  const accepted = report?.state === 'accepted'
  return <section className="fleet-panel review-panel" aria-labelledby="review-heading">
    <div className="panel-heading"><div><h2 id="review-heading">Nəticələr və təsdiq</h2><p>İş yalnız sizin təsdiqinizdən sonra tamamlanmış sayılır.</p></div><span className={`state ${accepted ? 'completed' : 'unknown'}`} role="status">{accepted ? 'Tamamlanıb · təsdiqlənib' : pending ? 'Təsdiq gözləyir' : report?.state === 'stale' ? 'Sübut köhnəlib' : 'Sübut gözlənilir'}</span></div>
    {error && <p className="review-message" role="alert">{error}</p>}
    {!report ? <p className="review-message">{unavailable ? 'Hazırda yoxlanmış layihə tapşırığı qoşulmayıb.' : 'Sübutlar yüklənir…'}</p> : <div className="review-body">
      <p><strong>Tapşırıq:</strong> {report.task_id}</p>
      <details><summary>Yoxlanmış məzmunun izi</summary><code>{report.revision}</code></details>
      <h3>Nəticə faylları</h3><ul>{report.artifacts.map(a => <li key={a.path}><strong>{a.path}</strong><code aria-label="Faylın SHA-256 izi">{a.sha256}</code>{report.previews?.filter(p => p.path === a.path).map(p => <details key={p.path}><summary>Fayla bax{p.truncated ? " · ilk hissə" : ""}</summary><pre>{p.text}</pre></details>)}</li>)}</ul>
      <h3>Test sübutları</h3><ul>{report.tests.map(t => <li key={t.id}><strong>{t.passed ? '✓ Keçdi' : '✕ Keçmədi'} · {t.id}</strong><span>{t.output_bytes} bayt test çıxışı yoxlanıb</span><code aria-label="Test çıxışının SHA-256 izi">{t.output_sha256}</code></li>)}</ul>
      <p>{accepted ? 'Bu nəticə ayrıca təsdiqləndi.' : report.state === 'stale' ? 'Fayllar dəyişib və ya sübutun vaxtı keçib. Yenidən yoxlama tələb olunur.' : `Faylları və test sübutlarını nəzərdən keçirin. Təsdiq müddəti: ${new Date(report.expires_at).toLocaleTimeString()}.`}</p>
      <button className="approve-button" disabled={!pending || busy || !!error} onClick={() => void approve()}>{busy ? 'Təsdiqlənir…' : 'Təsdiqlə və tamamla'}</button>
    </div>}
  </section>
}

function App() {
  const [fleet, setFleet] = useState<Fleet | null>(null)
  const [connection, setConnection] = useState<Connection>('connecting')
  const [provider, setProvider] = useState('all')
  const [query, setQuery] = useState('')
  const [now, setNow] = useState(Date.now())
  const [help, setHelp] = useState(false)

  useEffect(() => {
    let active = true
    let stream: EventSource | null = null
    const connect = () => {
      stream?.close()
      if (!navigator.onLine) { setConnection('reconnecting'); return }
      stream = new EventSource('/v0/events')
      stream.addEventListener('fleet', ((event: MessageEvent<string>) => {
        try {
          const next = JSON.parse(event.data) as Fleet
          if (!Array.isArray(next.sessions) || !Number.isSafeInteger(next.cursor)) throw new Error('Invalid snapshot')
          if (active) {
            setFleet(previous => previous && previous.cursor > next.cursor ? previous : next)
            setConnection('live')
          }
        } catch { if (active) setConnection('reconnecting') }
      }) as EventListener)
      stream.onerror = () => { if (active) setConnection('reconnecting') }
    }
    const offline = () => { stream?.close(); setConnection('reconnecting') }
    window.addEventListener('offline', offline)
    window.addEventListener('online', connect)
    connect()
    return () => {
      active = false
      stream?.close()
      window.removeEventListener('offline', offline)
      window.removeEventListener('online', connect)
    }
  }, [])

  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 1000); return () => window.clearInterval(timer) }, [])

  const sessions = fleet?.sessions ?? []
  const visible = sessions.filter(s => (provider === 'all' || s.provider === provider) && `${names[s.provider]} ${s.session_id} ${s.workspace_id}`.toLowerCase().includes(query.toLowerCase()))
  const running = sessions.filter(s => s.source_connected && s.execution_state === 'running').length
  const attention = sessions.filter(s => !s.source_connected || ['unknown', 'failed', 'interrupted'].includes(s.execution_state)).length
  const completed = sessions.filter(s => s.last_known_execution_state === 'completed').length

  return <>
    <a className="skip-link" href="#main">Skip to fleet</a>
    <header className="topbar">
      <a className="brand" href="/" aria-label="AgentOS home"><svg viewBox="0 0 28 28" width="28" height="28" aria-hidden="true"><rect width="28" height="28" rx="7" fill="currentColor"/><path d="M7 19 12 9h4l5 10h-4l-1-3h-4l-1 3Zm6-6h2l-1-3Z" fill="white"/></svg>AgentOS<span className="version">EARLY PREVIEW</span></a>
      <div className="local-mark"><span className="dot"/>Local workspace</div>
    </header>
    <div className="layout">
      <aside className="sidebar" aria-label="Workspace navigation">
        <div className="workspace"><span className="workspace-icon">A</span><div><strong>{fleet?.mode === 'demo' ? 'Demo workspace' : 'Your workspace'}</strong><small>On this computer</small></div></div>
        <nav aria-label="Main"><a className="nav-item selected" href="#main" aria-current="page"><span aria-hidden="true">▦</span> Live Fleet <span className="nav-count">{sessions.length}</span></a></nav>
        <div className="sidebar-note"><span className="small-label">LOCAL REVIEW</span><p>Agent observation and your explicit project-result approval. No provider commands are sent.</p></div>
        <a className="docs-link" href="https://github.com/Aazernajaf/agentos/blob/main/docs/architecture/live-fleet.md" target="_blank" rel="noreferrer">Setup & documentation <span aria-hidden="true">↗</span></a>
      </aside>
      <main id="main">
        <div className="breadcrumb">Workspace <span>/</span> Live Fleet</div>
        <div className="page-heading"><div><h1>Your agents, at a glance.</h1><p>See what is running, what has finished, and what needs a closer look.</p></div><span className={`connection ${connection}`} role="status"><span className="dot"/>{connection === 'live' ? 'Live connection' : connection === 'connecting' ? 'Connecting…' : 'Reconnecting…'}</span></div>
        {fleet?.mode === 'demo' && <div className="demo-banner"><span className="demo-icon" aria-hidden="true">◇</span><div><strong>You’re exploring a demo</strong><span>These are synthetic sessions. No real agent account is accessed.</span></div><span className="badge">DEMO</span></div>}
        {connection === 'reconnecting' && <div className="error-banner" role="alert"><span>Connection lost. Displayed observations may be out of date.</span><button onClick={() => window.location.reload()}>Reconnect</button></div>}
        <section className="metrics" aria-label="Fleet overview">
          {[['Observed sessions', sessions.length, 'Explicitly connected sources'], ['Running', running, 'Execution is in progress'], ['Execution complete', completed, 'Results still need verification'], ['Needs attention', attention, 'Failed, interrupted, or unknown']].map(([title, value, caption]) => <div className="metric" key={title}><span>{title}</span><strong>{fleet ? value : '—'}</strong><small>{caption}</small></div>)}
        </section>
        <ReviewPanel />
        <section className="fleet-panel" aria-labelledby="sessions-heading">
          <div className="panel-heading"><div><h2 id="sessions-heading">Agent sessions <span>{sessions.length}</span></h2><p>Execution state from your connected sources.</p></div><button className="secondary" aria-expanded={help} aria-controls="connection-help" onClick={() => setHelp(!help)}>How to connect <span aria-hidden="true">↗</span></button></div>
          {help && <div id="connection-help" className="help"><strong>Connect an explicitly selected session</strong><p>This preview reads an owned provider’s event stream from standard input. It does not scan private histories or launch agents.</p><a href="https://github.com/Aazernajaf/agentos/blob/main/docs/architecture/live-fleet.md" target="_blank" rel="noreferrer">Read connection instructions ↗</a></div>}
          <div className="filters"><label className="search"><span aria-hidden="true">⌕</span><span className="sr-only">Search sessions</span><input type="search" placeholder="Search sessions or workspace…" value={query} onChange={e => setQuery(e.target.value)} /></label><div className="provider-filter"><label htmlFor="provider">Provider</label><select id="provider" value={provider} onChange={e => setProvider(e.target.value)}><option value="all">All providers</option><option value="codex">Codex</option><option value="claude">Claude Code</option></select></div></div>
          {!fleet ? <div className="empty" role="status"><h3>Waiting for a local connection</h3><p>Session observations will appear here when the connection is ready.</p>{connection === 'reconnecting' && <button onClick={() => window.location.reload()}>Try again</button>}</div> : visible.length === 0 ? <div className="empty"><h3>{sessions.length ? 'No matching sessions' : 'No sessions observed yet'}</h3><p>{sessions.length ? 'Try another provider or clear your search.' : 'Connect an explicitly selected session to see its execution state.'}</p><button onClick={() => { if (sessions.length) { setQuery(''); setProvider('all') } else setHelp(true) }}>{sessions.length ? 'Clear filters' : 'Show connection instructions'}</button></div> : <div className="table-scroll" role="region" aria-label="Agent sessions; scroll horizontally for all columns" tabIndex={0}><table><caption className="sr-only">Observed agent execution state; completion is not verified task acceptance</caption><thead><tr><th scope="col">Agent</th><th scope="col">Execution state</th><th scope="col">Workspace</th><th scope="col">Last observation</th></tr></thead><tbody>{visible.map(s => {
            const stale = connection !== 'live' || !s.source_connected || now - Date.parse(s.last_seen) > 30000
            return <tr key={identity(s)}><td><div className="agent-cell"><span className={`agent-icon ${s.provider}`} aria-hidden="true">{s.provider === 'codex' ? 'C' : '✳'}</span><div><strong>{names[s.provider]}</strong><span className="session-id" title={s.session_id}>{s.session_id}</span><small>v{s.provider_version}</small></div></div></td><td><span className={`state ${s.execution_state}`}><span className="dot"/>{states[s.execution_state]}</span>{!s.source_connected && <small className="state-note">Source disconnected{s.last_known_execution_state ? ` · Last: ${s.last_known_execution_state}` : ''}</small>}</td><td><span className="workspace-name">{s.workspace_id}</span><small className="installation">{s.installation_id}</small></td><td><time dateTime={s.last_seen}>{ageLabel(s.last_seen, now)}</time>{stale && <small className="state-note">Observation may be stale</small>}</td></tr>
          })}</tbody></table></div>}
          <div className="panel-footer"><span>{visible.length} of {sessions.length} sessions</span><span>Read-only · No task acceptance inferred</span></div>
        </section>
        <div className="bottom-note"><span aria-hidden="true">ⓘ</span><p>Activity is only part of the picture. A finished execution still needs evidence and review before its work can be accepted.</p></div>
        <footer>AgentOS <span>Local-first. Outcome-focused.</span><span>In-memory preview · History resets on restart</span></footer>
      </main>
    </div>
  </>
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>)
