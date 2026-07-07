import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { api, type Domain } from '../api/client'
import { Input, Text, Surface, Loader, Empty } from '@cloudflare/kumo'

export const Route = createFileRoute('/')({
  component: Home,
})

function Home() {
  const { data: domains, isLoading } = useQuery({
    queryKey: ['domains'],
    queryFn: api.domains,
  })

  return (
    <div>
      <div style={{ textAlign: 'center', marginBottom: 40 }}>
        <Text size="2xl" weight="bold" as="h1" style={{ marginBottom: 8 }}>知识池</Text>
        <Text color="secondary">浏览所有知识领域，搜索理念</Text>

        <form
          style={{ marginTop: 24 }}
          onSubmit={(e) => {
            e.preventDefault()
            const q = (e.currentTarget.elements.namedItem('q') as HTMLInputElement).value
            if (q.trim()) {
              window.location.href = `/search?q=${encodeURIComponent(q.trim())}`
            }
          }}
        >
          <Input name="q" placeholder="搜索概念…" style={{ width: 360 }} />
        </form>
      </div>

      {isLoading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Loader /></div>
      ) : (domains ?? []).length === 0 ? (
        <Empty title="暂无领域" description="知识池中没有数据" />
      ) : (
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
          gap: 12,
        }}>
          {(domains ?? []).map((d: Domain) => (
            <a key={d.domain} href={`/domain/${d.domain}`} style={{ textDecoration: 'none' }}>
              <Surface style={{
                padding: '20px 24px',
                cursor: 'pointer',
                transition: 'border-color 0.15s',
              }}>
                <Text weight="medium">{d.domain}</Text>
                <Text size="sm" color="secondary" style={{ marginTop: 4 }}>
                  {d.concept_count} concepts
                </Text>
              </Surface>
            </a>
          ))}
        </div>
      )}
    </div>
  )
}
