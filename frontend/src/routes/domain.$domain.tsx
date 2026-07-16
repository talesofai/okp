import { createFileRoute, Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import {
  useApi,
  type Concept,
  type CreateInviteResponse,
  type DomainInviteRow,
} from "../api/client";
import { Text, Badge, Surface, Empty, Button } from "@cloudflare/kumo";

declare const marked: { parse: (md: string) => string };

/** Strip YAML frontmatter (--- ... ---) from markdown, return body only. */
function stripFrontmatter(md: string): string {
  if (!md) return "";
  const trimmed = md.trimStart();
  if (!trimmed.startsWith("---")) return md;
  const end = trimmed.indexOf("\n---", 3);
  if (end === -1) return md;
  return trimmed.slice(end + 4).trimStart();
}

const markdownStyles = `
  .okp-md h1 { font-size: 1.4em; font-weight: 600; margin: 0.6em 0 0.3em; }
  .okp-md h2 { font-size: 1.2em; font-weight: 600; margin: 0.6em 0 0.3em; }
  .okp-md h3 { font-size: 1.1em; font-weight: 600; margin: 0.5em 0 0.2em; }
  .okp-md p { margin: 0.4em 0; }
  .okp-md ul, .okp-md ol { margin: 0.4em 0; padding-left: 1.4em; }
  .okp-md li { margin: 0.2em 0; }
  .okp-md code { background: rgba(128,128,128,0.15); padding: 1px 5px; border-radius: 3px; font-size: 0.88em; font-family: ui-monospace, monospace; }
  .okp-md pre { background: rgba(128,128,128,0.1); padding: 12px; border-radius: 6px; overflow-x: auto; margin: 0.6em 0; }
  .okp-md pre code { background: none; padding: 0; }
  .okp-md a { color: #6366f1; }
  .okp-md strong { font-weight: 600; }
  .okp-md blockquote { border-left: 3px solid rgba(128,128,128,0.3); padding-left: 12px; margin: 0.5em 0; opacity: 0.8; }
  .okp-md table { border-collapse: collapse; margin: 0.5em 0; }
  .okp-md th, .okp-md td { border: 1px solid rgba(128,128,128,0.2); padding: 4px 10px; }
  .okp-md hr { border: none; border-top: 1px solid rgba(128,128,128,0.2); margin: 0.8em 0; }
`;

export const Route = createFileRoute("/domain/$domain")({
  component: DomainPage,
})

const PAGE_SIZE = 50

function DomainPage() {
  const api = useApi();
  const queryClient = useQueryClient();
  const { domain } = Route.useParams()
  const [offset, setOffset] = useState(0)
  const [readmeCollapsed, setReadmeCollapsed] = useState(false)
  const [createdInvite, setCreatedInvite] = useState<CreateInviteResponse | null>(null)
  const [inviteError, setInviteError] = useState<string | null>(null)

  const { data: concepts, isLoading } = useQuery({
    queryKey: ['domain', domain, offset],
    queryFn: () => api.search({ domain, limit: PAGE_SIZE, offset }),
  })

  const { data: meta } = useQuery({
    queryKey: ['domain-meta', domain],
    queryFn: () => api.getDomainReadme(domain),
    retry: false,
  })

  const { data: me } = useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    retry: false,
  })

  const myDomainRole = useMemo(() => {
    if (me?.role === 'admin') return 'admin'
    const m = me?.domains?.find((d) => d.domain === domain)
    return m?.role ?? 'reader'
  }, [me, domain])

  const canManage = myDomainRole === 'admin' || myDomainRole === 'host'

  const { data: invites } = useQuery({
    queryKey: ['domain-invites', domain],
    queryFn: () => api.listInvites(domain),
    enabled: canManage,
    retry: false,
  })

  const createInviteMutation = useMutation({
    mutationFn: () => api.createInvite(domain, { role: 'writer', expires_in_hours: 72, max_uses: 1 }),
    onSuccess: (res) => {
      setCreatedInvite(res)
      setInviteError(null)
      queryClient.invalidateQueries({ queryKey: ['domain-invites', domain] })
    },
    onError: (err: Error) => {
      setInviteError(err.message || '创建邀请失败')
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) => api.revokeInvite(domain, id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['domain-invites', domain] })
    },
  })

  const readmeHtml = useMemo(() => {
    const raw = meta?.readme ?? ""
    const body = stripFrontmatter(raw)
    if (!body) return ""
    return typeof marked !== 'undefined'
      ? marked.parse(body)
      : `<pre>${body}</pre>`
  }, [meta?.readme])

  const hasReadme = Boolean(readmeHtml)

  return (
    <div>
      <style dangerouslySetInnerHTML={{ __html: markdownStyles }} />

      <Link to="/" style={{ textDecoration: "none", marginBottom: 16, display: "inline-block" }}>
        <Text size="sm" color="secondary">← 返回</Text>
      </Link>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16, flexWrap: 'wrap' }}>
        <Text size="xl" weight="bold" as="h1">{domain}</Text>
        {concepts && (
          <Badge variant="info">{concepts.length} on this page</Badge>
        )}
        <Badge variant="default">{myDomainRole}</Badge>
        {hasReadme && (
          <Button
            variant="outline"
            onClick={() => setReadmeCollapsed(!readmeCollapsed)}
          >
            {readmeCollapsed ? '展开说明' : '收起说明'}
          </Button>
        )}
      </div>

      {hasReadme && !readmeCollapsed && (
        <Surface style={{
          padding: "20px 24px",
          marginBottom: 24,
          borderLeft: "3px solid #6366f1",
        }}>
          <div
            className="okp-md"
            style={{ lineHeight: 1.7, fontSize: 14 }}
            dangerouslySetInnerHTML={{ __html: readmeHtml }}
          />
        </Surface>
      )}

      {canManage && (
        <Surface style={{ padding: 20, marginBottom: 24 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
            <div>
              <Text weight="medium">邀请成员</Text>
              <Text size="sm" color="secondary">
                生成 writer 邀请码（默认 72 小时、一次性）。对方打开固定 Work 链接后在首页输入邀请码。
              </Text>
            </div>
            <Button
              onClick={() => createInviteMutation.mutate()}
              disabled={createInviteMutation.isPending}
            >
              {createInviteMutation.isPending ? '生成中…' : '生成邀请码'}
            </Button>
          </div>

          {inviteError && (
            <Text size="sm" color="danger" style={{ marginTop: 12 }}>{inviteError}</Text>
          )}

          {createdInvite && (
            <div style={{
              marginTop: 16,
              padding: 12,
              borderRadius: 8,
              background: 'rgba(99,102,241,0.08)',
              display: 'flex',
              flexDirection: 'column',
              gap: 8,
            }}>
              <Text size="sm" color="secondary">邀请码（只显示一次）</Text>
              <Text weight="medium" style={{ fontFamily: 'ui-monospace, monospace', fontSize: 18 }}>
                {createdInvite.code}
              </Text>
              <pre style={{
                margin: 0,
                whiteSpace: 'pre-wrap',
                fontSize: 12,
                fontFamily: 'ui-monospace, monospace',
                opacity: 0.9,
              }}>{createdInvite.share_text}</pre>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <Button
                  variant="outline"
                  onClick={() => navigator.clipboard?.writeText(createdInvite.code)}
                >
                  复制邀请码
                </Button>
                <Button
                  variant="outline"
                  onClick={() => navigator.clipboard?.writeText(createdInvite.share_text)}
                >
                  复制邀请文案
                </Button>
              </div>
            </div>
          )}

          {invites && invites.length > 0 && (
            <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 8 }}>
              <Text size="sm" color="secondary">最近邀请</Text>
              {invites.slice(0, 8).map((inv: DomainInviteRow) => (
                <div key={inv.id} style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 8,
                  alignItems: 'center',
                  fontSize: 13,
                }}>
                  <Text size="sm">
                    {inv.role} · {inv.status} · {inv.used_count}/{inv.max_uses}
                  </Text>
                  {inv.status === 'active' && (
                    <Button
                      variant="outline"
                      onClick={() => revokeMutation.mutate(inv.id)}
                      disabled={revokeMutation.isPending}
                    >
                      撤销
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
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
