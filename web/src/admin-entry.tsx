import React, { StrictMode, useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'

type Page = 'users' | 'projects'

type ApiList<T> = {
  items: T[]
  total: number
  offset: number
  limit: number
}

type User = {
  ID: number
  Email: string
  Name: string
  Role: 'admin' | 'editor' | 'viewer'
  IsActive: boolean
  CreatedAt: string
  UpdatedAt: string
}

type Project = {
  ID: number
  Key: string
  Name: string
  Status: 'active' | 'paused' | 'archived'
  Description: string
  CreatedAt: string
  UpdatedAt: string
}

async function apiJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const j = await res.json()
      msg = j?.error ?? msg
    } catch {
      // ignore
    }
    throw new Error(msg)
  }
  return (await res.json()) as T
}

function usePager(initialLimit = 50) {
  const [offset, setOffset] = useState(0)
  const [limit, setLimit] = useState(initialLimit)
  return { offset, setOffset, limit, setLimit }
}

function Pager({
  total,
  offset,
  limit,
  onOffset,
}: {
  total: number
  offset: number
  limit: number
  onOffset: (n: number) => void
}) {
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil(total / limit))
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
      <button disabled={offset <= 0} onClick={() => onOffset(Math.max(0, offset - limit))}>
        Prev
      </button>
      <button
        disabled={offset + limit >= total}
        onClick={() => onOffset(Math.min((pages - 1) * limit, offset + limit))}
      >
        Next
      </button>
      <span style={{ opacity: 0.8 }}>
        Page {page} / {pages} ({total} items)
      </span>
    </div>
  )
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div
      style={{
        border: '1px solid rgba(35,52,90,.9)',
        borderRadius: 14,
        background: 'rgba(15,25,48,.55)',
        padding: 12,
      }}
    >
      {children}
    </div>
  )
}

function UsersAdmin() {
  const [q, setQ] = useState('')
  const [role, setRole] = useState('')
  const pager = usePager(50)
  const [data, setData] = useState<ApiList<User> | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const query = useMemo(() => {
    const sp = new URLSearchParams()
    if (q) sp.set('q', q)
    if (role) sp.set('role', role)
    sp.set('offset', String(pager.offset))
    sp.set('limit', String(pager.limit))
    return sp.toString()
  }, [q, role, pager.offset, pager.limit])

  useEffect(() => {
    let alive = true
    setLoading(true)
    setErr(null)
    apiJSON<ApiList<User>>(`/api/users?${query}`)
      .then((j) => {
        if (!alive) return
        setData(j)
      })
      .catch((e: unknown) => {
        if (!alive) return
        setErr(e instanceof Error ? e.message : 'error')
      })
      .finally(() => {
        if (!alive) return
        setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [query])

  async function setUserRole(id: number, newRole: User['Role']) {
    await apiJSON(`/api/users/${id}/role`, { method: 'POST', body: JSON.stringify({ role: newRole }) })
    // refresh
    const j = await apiJSON<ApiList<User>>(`/api/users?${query}`)
    setData(j)
  }
  async function setUserActive(id: number, active: boolean) {
    await apiJSON(`/api/users/${id}/active`, {
      method: 'POST',
      body: JSON.stringify({ active }),
    })
    const j = await apiJSON<ApiList<User>>(`/api/users?${query}`)
    setData(j)
  }
  async function resetPassword(id: number) {
    const pw = window.prompt('新しいパスワードを入力', 'admin')
    if (!pw) return
    await apiJSON(`/api/users/${id}/reset-password`, {
      method: 'POST',
      body: JSON.stringify({ password: pw }),
    })
    window.alert('更新しました')
  }

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <Card>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              pager.setOffset(0)
            }}
            placeholder="Search (email/name)"
            style={{
              flex: '1 1 240px',
              padding: '8px 10px',
              borderRadius: 10,
              border: '1px solid rgba(35,52,90,.9)',
              background: 'rgba(16,27,51,.45)',
              color: '#e8eefc',
            }}
          />
          <select
            value={role}
            onChange={(e) => {
              setRole(e.target.value)
              pager.setOffset(0)
            }}
            style={{
              padding: '8px 10px',
              borderRadius: 10,
              border: '1px solid rgba(35,52,90,.9)',
              background: 'rgba(16,27,51,.45)',
              color: '#e8eefc',
            }}
          >
            <option value="">All roles</option>
            <option value="admin">admin</option>
            <option value="editor">editor</option>
            <option value="viewer">viewer</option>
          </select>
          {data ? (
            <Pager total={data.total} offset={data.offset} limit={data.limit} onOffset={pager.setOffset} />
          ) : null}
        </div>
        {loading ? <div style={{ marginTop: 10, opacity: 0.8 }}>Loading...</div> : null}
        {err ? (
          <div style={{ marginTop: 10, color: '#ffd6d6' }}>
            Error: {err}（admin権限が必要な操作もあります）
          </div>
        ) : null}
      </Card>

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '1px solid rgba(35,52,90,.9)' }}>
              <th style={{ padding: '10px 8px' }}>ID</th>
              <th style={{ padding: '10px 8px' }}>Email</th>
              <th style={{ padding: '10px 8px' }}>Name</th>
              <th style={{ padding: '10px 8px' }}>Role</th>
              <th style={{ padding: '10px 8px' }}>Active</th>
              <th style={{ padding: '10px 8px' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((u) => (
              <tr key={u.ID} style={{ borderBottom: '1px solid rgba(35,52,90,.55)' }}>
                <td style={{ padding: '10px 8px', opacity: 0.85 }}>{u.ID}</td>
                <td style={{ padding: '10px 8px' }}>{u.Email}</td>
                <td style={{ padding: '10px 8px' }}>{u.Name}</td>
                <td style={{ padding: '10px 8px' }}>
                  <select
                    value={u.Role}
                    onChange={(e) => setUserRole(u.ID, e.target.value as User['Role'])}
                  >
                    <option value="admin">admin</option>
                    <option value="editor">editor</option>
                    <option value="viewer">viewer</option>
                  </select>
                </td>
                <td style={{ padding: '10px 8px' }}>
                  <input
                    type="checkbox"
                    checked={u.IsActive}
                    onChange={(e) => setUserActive(u.ID, e.target.checked)}
                  />
                </td>
                <td style={{ padding: '10px 8px' }}>
                  <button onClick={() => resetPassword(u.ID)}>Reset PW</button>
                </td>
              </tr>
            ))}
            {!loading && (data?.items?.length ?? 0) === 0 ? (
              <tr>
                <td colSpan={6} style={{ padding: '14px 8px', opacity: 0.8 }}>
                  No users
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function ProjectsAdmin() {
  const [q, setQ] = useState('')
  const [status, setStatus] = useState('')
  const pager = usePager(50)
  const [data, setData] = useState<ApiList<Project> | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const [newKey, setNewKey] = useState('')
  const [newName, setNewName] = useState('')
  const [newStatus, setNewStatus] = useState<Project['Status']>('active')

  const query = useMemo(() => {
    const sp = new URLSearchParams()
    if (q) sp.set('q', q)
    if (status) sp.set('status', status)
    sp.set('offset', String(pager.offset))
    sp.set('limit', String(pager.limit))
    return sp.toString()
  }, [q, status, pager.offset, pager.limit])

  async function refresh() {
    const j = await apiJSON<ApiList<Project>>(`/api/projects?${query}`)
    setData(j)
  }

  useEffect(() => {
    let alive = true
    setLoading(true)
    setErr(null)
    apiJSON<ApiList<Project>>(`/api/projects?${query}`)
      .then((j) => {
        if (!alive) return
        setData(j)
      })
      .catch((e: unknown) => {
        if (!alive) return
        setErr(e instanceof Error ? e.message : 'error')
      })
      .finally(() => {
        if (!alive) return
        setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [query])

  async function createProject() {
    setErr(null)
    try {
      await apiJSON<Project>('/api/projects', {
        method: 'POST',
        body: JSON.stringify({
          key: newKey,
          name: newName,
          status: newStatus,
          description: '',
        }),
      })
      setNewKey('')
      setNewName('')
      await refresh()
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : 'error')
    }
  }

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <Card>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              pager.setOffset(0)
            }}
            placeholder="Search (key/name)"
            style={{
              flex: '1 1 240px',
              padding: '8px 10px',
              borderRadius: 10,
              border: '1px solid rgba(35,52,90,.9)',
              background: 'rgba(16,27,51,.45)',
              color: '#e8eefc',
            }}
          />
          <select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value)
              pager.setOffset(0)
            }}
          >
            <option value="">All status</option>
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="archived">archived</option>
          </select>
          {data ? (
            <Pager total={data.total} offset={data.offset} limit={data.limit} onOffset={pager.setOffset} />
          ) : null}
        </div>
        {loading ? <div style={{ marginTop: 10, opacity: 0.8 }}>Loading...</div> : null}
        {err ? <div style={{ marginTop: 10, color: '#ffd6d6' }}>Error: {err}</div> : null}
      </Card>

      <Card>
        <div style={{ fontWeight: 700, marginBottom: 8 }}>Create project</div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'center' }}>
          <input
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder="key (unique)"
          />
          <input value={newName} onChange={(e) => setNewName(e.target.value)} placeholder="name" />
          <select value={newStatus} onChange={(e) => setNewStatus(e.target.value as Project['Status'])}>
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="archived">archived</option>
          </select>
          <button onClick={createProject}>Create</button>
        </div>
        <div style={{ marginTop: 8, opacity: 0.8, fontSize: 12 }}>
          admin/editor が作成可能（viewer は不可）
        </div>
      </Card>

      <div style={{ overflowX: 'auto' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '1px solid rgba(35,52,90,.9)' }}>
              <th style={{ padding: '10px 8px' }}>ID</th>
              <th style={{ padding: '10px 8px' }}>Key</th>
              <th style={{ padding: '10px 8px' }}>Name</th>
              <th style={{ padding: '10px 8px' }}>Status</th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((p) => (
              <tr key={p.ID} style={{ borderBottom: '1px solid rgba(35,52,90,.55)' }}>
                <td style={{ padding: '10px 8px', opacity: 0.85 }}>{p.ID}</td>
                <td style={{ padding: '10px 8px' }}>{p.Key}</td>
                <td style={{ padding: '10px 8px' }}>{p.Name}</td>
                <td style={{ padding: '10px 8px' }}>{p.Status}</td>
              </tr>
            ))}
            {!loading && (data?.items?.length ?? 0) === 0 ? (
              <tr>
                <td colSpan={4} style={{ padding: '14px 8px', opacity: 0.8 }}>
                  No projects
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function App() {
  const root = document.getElementById('react-admin-root') as HTMLElement | null
  const page = (root?.dataset?.page ?? '') as Page
  if (!root) return null
  return (
    <div style={{ display: 'grid', gap: 12 }}>
      {page === 'users' ? <UsersAdmin /> : null}
      {page === 'projects' ? <ProjectsAdmin /> : null}
      {page !== 'users' && page !== 'projects' ? (
        <div style={{ opacity: 0.8 }}>Unknown page: {String(page)}</div>
      ) : null}
    </div>
  )
}

const mount = document.getElementById('react-admin-root')
if (mount) {
  createRoot(mount).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

