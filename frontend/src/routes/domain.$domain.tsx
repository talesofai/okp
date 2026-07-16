import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useApi, type Concept } from "../api/client";
import { Text, Badge, Surface, Empty, Button } from "@cloudflare/kumo";

declare const marked: { parse: (md: string) => string };

export const Route = createFileRoute("/domain/$domain")({
  component: DomainPage,
})

const PAGE_SIZE = 50

function DomainPage() {
  const api = useApi();
  const { domain } = Route.useParams()
  const [offset, setOffset] = useState(0)
  const [showReadme, setShowReadme] = useState(false)

  const { data: concepts, isLoading } = useQuery({
    queryKey: ['domain', domain, offset],
    queryFn: () => api.search({ domain, limit: PAGE_SIZE, offset }),
  })

  const { data: meta } = useQuery({
    queryKey: ['domain-meta', domain],
    queryFn: () => api.getDomainReadme(domain),
    retry: false,
  })

  const hasReadme = meta?.readme && meta.readme.trim().length > 0

  return (
    <div>
      <Link to="/" style={{ textDecoration: "none", marginBottom: 16, display: "inline-block" }}>
        <Text size="sm" color="secondary">← 返回</Text>
      </Link>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 8 }}>
        <Text size="xl" weight="bold" as="h1">{domain}</Text>
        {concepts && (
          <Badge variant="info">{concepts.length} concepts on this page</Badge>
        )}
        {hasReadme && (
          <Button variant="outline" onClick={() => setShowReadme(!showReadme)}>
            {showReadme ? '隐藏 README' : '查看 README'}
          </Button>
        )}
      </div>

      {showReadme && hasReadme && (
        <Surface style={{ padding: 24, marginBottom: 24, maxHeight: 400, overflow: 'auto' }}>
          <div
            className="markdown-body"
            style={{ lineHeight: 1.7 }}
            ref={(el) => {
              if (el && meta?.readme) {
                el.innerHTML = typeof marked !== 'undefined'
                  ? marked.parse(meta.readme)
                  : `<pre>${meta.readme}</pre>`
              }
            }}
          />
        </Surface>
      )}

      {isLoading && (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Text color="secondary">loading…</Text>
        </div>
      )}

      {concepts && concepts.length === 0 && !isLoading && (
        <Empty title="暂无数据" description="该领域下没有概念" />
      )}

      {concepts && concepts.length > 0 && (
        <>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {concepts.map((c: Concept) => (
              <Link key={c.id} to="/concept/$" params={{ _splat: c.id }} style={{ textDecoration: "none" }}>
                <Surface style={{
                  padding: "16px 20px",
                  cursor: "pointer",
                  transition: "border-color 0.15s",
                }}>
                  <Text size="xs" color="secondary" style={{ marginBottom: 4 }}>
                    {c.type}
                  </Text>
                  <Text weight="medium">{c.title}</Text>
                  {c.description && (
                    <Text size="sm" color="secondary" style={{
                      marginTop: 4,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
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
              </Link>
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
