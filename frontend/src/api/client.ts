const BASE = 'https://okp.neta.art'
const TOKEN = 'tok'

const headers = {
  'Content-Type': 'application/json',
  'Authorization': `Bearer ${TOKEN}`,
}

export interface Domain {
  domain: string
  concept_count: number
}

export interface Concept {
  id: string
  domain: string
  type: string
  title: string
  description: string
  tags: string[]
  body?: string
  resource?: string
  status: string
  frontmatter?: Record<string, string>
  provenance?: Record<string, string>
  content_hash?: string
  created_at?: string
  updated_at?: string
  match_reason?: string
}

export interface SearchResult {
  concepts: Concept[]
  total: number
}

async function fetchOkp<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { headers })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || `HTTP ${res.status}`)
  }
  return res.json()
}

export const api = {
  health: () => fetchOkp<{ status: string }>('/api/v1/health'),

  domains: () => fetchOkp<Domain[]>('/api/v1/domains'),

  search: (params: {
    q?: string
    domain?: string
    type?: string
    tag?: string
    limit?: number
    offset?: number
  }) => {
    const sp = new URLSearchParams()
    if (params.q) sp.set('q', params.q)
    if (params.domain) sp.set('domain', params.domain)
    if (params.type) sp.set('type', params.type)
    if (params.tag) sp.set('tag', params.tag)
    sp.set('limit', String(params.limit ?? 50))
    if (params.offset) sp.set('offset', String(params.offset))
    sp.set('status', 'accepted')
    return fetchOkp<Concept[]>(`/api/v1/concepts?${sp}`)
  },

  getConcept: (id: string) =>
    fetchOkp<Concept>(`/api/v1/concepts/${encodeURIComponent(id)}`),

  getLinks: (id: string) =>
    fetchOkp<{ outgoing: unknown[]; backlinks: unknown[] }>(
      `/api/v1/links/${encodeURIComponent(id)}`
    ),
}
