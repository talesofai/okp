import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { api, type Concept } from '../api/client'
import { Text, Badge, Surface, Loader, Empty, Button } from '@cloudflare/kumo'

export const Route = createFileRoute('/domain/$domain')({
  component: DomainPage,
})

const PAGE_SIZE = 50

function DomainPage() {
  const { domain } = Route.useParams()
  const [offset, setOffset] = useState(0)

  const { data: concepts, isLoading } = useQuery({
    queryKey: ['domain', domain, offset],
    queryFn: () => api.search({ domain, limit: PAGE_SIZE, offset }),
  })

  return (
    <div>
      <a href="/" style={{ textDecoration: 'none', marginBottom: 16, display: 'inline-block' }}>
        <Text size="sm" color="secondary">← 返回</Text>
      </a>
      <Text size="xl" weight="bold" as="h1" style={{ marginBottom: 24 }}>{domain}</Text>

      {isLoading && (
        <div style={{ textAlign: 'center', padding: 48 }}><Loader /></div>
      )}

      {concepts && concepts.length === 0 && !isLoading && (
        <Empty title="暂无数据" description="该领域下没有概念" />
      )}

      {concepts && concepts.length > 0 && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {concepts.map((c: Concept) => (
              <a key={c.id} href={`/concept/${c.id}`} style={{ textDecoration: 'none' }}>
                <Surface style={{
                  padding: '16px 20px',
                  cursor: 'pointer',
                  transition: 'border-color 0.15s',
                }}>
                  <Text size="xs" color="secondary" style={{ marginBottom: 4 }}>
                    {c.type}
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
                  {c.tags && c.tags.length > 0 && (
                    <div style={{ display: 'flex', gap: 4, marginTop: 8, flexWrap: 'wrap' }}>
                      {c.tags.slice(0, 5).map((t) => (
                        <Badge key={t} variant="default">{t}</Badge>
                      ))}
                    </div>
                  )}
                </Surface>
              </a>
            ))}
          </div>

          <div style={{ display: 'flex', justifyContent: 'center', gap: 8, marginTop: 24 }}>
            <Button
              variant="outline"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
            >
              上一页
            </Button>
            <Button
              variant="outline"
              disabled={!concepts || concepts.length < PAGE_SIZE}
              onClick={() => setOffset(offset + PAGE_SIZE)}
            >
              下一页
            </Button>
          </div>
        </>
      )}
    </div>
  )
}
