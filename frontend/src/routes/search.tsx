import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { api, type Concept } from '../api/client'
import { Input, Text, Badge, Surface, Loader, Empty, Button } from '@cloudflare/kumo'

export const Route = createFileRoute('/search')({
  component: Search,
  validateSearch: (s: Record<string, unknown>) => ({
    q: (s.q as string) ?? '',
  }),
})

function Search() {
  const { q } = Route.useSearch()

  const { data: results, isLoading } = useQuery({
    queryKey: ['search', q],
    queryFn: () => api.search({ q, limit: 50 }),
    enabled: q.length > 0,
  })

  return (
    <div>
      <form
        style={{ marginBottom: 32 }}
        onSubmit={(e) => {
          e.preventDefault()
          const v = (e.currentTarget.elements.namedItem('q') as HTMLInputElement).value
          if (v.trim()) {
            window.location.href = `/search?q=${encodeURIComponent(v.trim())}`
          }
        }}
      >
        <div style={{ display: 'flex', gap: 8 }}>
          <Input name="q" defaultValue={q} placeholder="搜索…" style={{ flex: 1 }} />
          <Button type="submit">搜索</Button>
        </div>
      </form>

      {isLoading && (
        <div style={{ textAlign: 'center', padding: 48 }}><Loader /></div>
      )}

      {results && results.length === 0 && !isLoading && (
        <Empty title="无结果" description={`未找到 "${q}" 的相关概念`} />
      )}

      {results && results.length > 0 && (
        <div>
          <Text size="sm" color="secondary" style={{ marginBottom: 16 }}>
            {results.length} 条结果
          </Text>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {results.map((c: Concept) => (
              <ConceptCard key={c.id} concept={c} />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function ConceptCard({ concept: c }: { concept: Concept }) {
  return (
    <a href={`/concept/${c.id}`} style={{ textDecoration: 'none' }}>
      <Surface style={{
        padding: '16px 20px',
        cursor: 'pointer',
        transition: 'border-color 0.15s',
      }}>
        <Text size="xs" color="secondary" style={{ marginBottom: 4 }}>
          {c.domain}/{c.type}
        </Text>
        <Text weight="medium">{c.title}</Text>
        {c.description && (
          <Text size="sm" color="secondary" style={{
            marginTop: 4,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {c.description.slice(0, 150)}
          </Text>
        )}
        <div style={{ display: 'flex', gap: 4, marginTop: 8, flexWrap: 'wrap' }}>
          {c.tags?.slice(0, 5).map((t) => (
            <Badge key={t} variant="default">{t}</Badge>
          ))}
          {c.match_reason && (
            <Badge variant="info">{c.match_reason}</Badge>
          )}
        </div>
      </Surface>
    </a>
  )
}
