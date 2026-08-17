import { createFileRoute, useNavigate, Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useApi, type Domain } from "../api/client";
import { useState } from "react";
import { Input, Text, Surface, Button, Badge } from "@cloudflare/kumo";
import { formatCompactDate, formatDateTime, formatRelativeTime } from "../lib/time";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [readme, setReadme] = useState("");
  const [visibility, setVisibility] = useState<"public" | "private">("public");

  const { data: domains, isLoading } = useQuery({
    queryKey: ["domains"],
    queryFn: api.domains,
  });

  const createMutation = useMutation({
    mutationFn: () => api.putDomain(name.trim(), { readme: readme.trim(), visibility }),
    onSuccess: async (domain) => {
      await queryClient.invalidateQueries({ queryKey: ["domains"] });
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      setCreateOpen(false);
      setName("");
      setReadme("");
      setVisibility("public");
      navigate({ to: "/domain/$domain", params: { domain: domain.domain } });
    },
  });

  const validName = /^[A-Za-z0-9][A-Za-z0-9_-]*$/.test(name.trim());

  return (
    <div>
      <div style={{ marginBottom: 40 }}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "flex-start", flexWrap: "wrap" }}>
          <div>
            <Text size="2xl" weight="bold" as="h1" style={{ marginBottom: 8 }}>Open Knowledge Protocol</Text>
            <Text color="secondary">Browse domains, search concepts</Text>
          </div>
          <Button onClick={() => setCreateOpen(true)}>创建 Domain</Button>
        </div>

        <form
          style={{ marginTop: 24, maxWidth: 520 }}
          onSubmit={(e) => {
            e.preventDefault();
            const q = (e.currentTarget.elements.namedItem("q") as HTMLInputElement).value;
            if (q.trim()) {
              navigate({ to: "/search", search: { q: q.trim() } });
            }
          }}
        >
          <Input name="q" placeholder="搜索概念…" style={{ width: "100%" }} />
        </form>
      </div>

      {createOpen && (
        <div
          role="dialog"
          aria-modal="true"
          style={{
            position: "fixed", inset: 0, zIndex: 1000, padding: 16,
            background: "rgba(0,0,0,0.45)", display: "flex",
            alignItems: "center", justifyContent: "center",
          }}
          onClick={() => setCreateOpen(false)}
        >
          <Surface
            style={{ width: "min(560px, 100%)", padding: 24, display: "flex", flexDirection: "column", gap: 16 }}
            onClick={(e) => e.stopPropagation()}
          >
            <div>
              <Text weight="medium">创建 Domain</Text>
              <Text size="sm" color="secondary">创建者自动成为唯一 host。</Text>
            </div>
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <Text size="sm">名称</Text>
              <Input value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="my-domain" autoFocus />
            </label>
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <Text size="sm">可见性</Text>
              <div style={{ display: "flex", gap: 8 }}>
                <Button variant={visibility === "public" ? "primary" : "outline"} onClick={() => setVisibility("public")}>公开</Button>
                <Button variant={visibility === "private" ? "primary" : "outline"} onClick={() => setVisibility("private")}>私有</Button>
              </div>
              <Text size="xs" color="secondary">
                {visibility === "private" ? "只有显式邀请的 reader/writer 可以发现和读取。" : "所有认证用户可发现和读取。"}
              </Text>
            </div>
            <label style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <Text size="sm">README</Text>
              <textarea
                value={readme}
                onChange={(e) => setReadme(e.currentTarget.value)}
                placeholder="# my-domain\n\n这个领域包含…"
                rows={10}
                style={{
                  width: "100%", resize: "vertical", padding: 12, borderRadius: 6,
                  border: "1px solid rgba(128,128,128,0.35)", background: "transparent",
                  color: "inherit", font: "13px/1.5 ui-monospace, monospace",
                }}
              />
            </label>
            {createMutation.error && <Text size="sm" color="danger">{(createMutation.error as Error).message}</Text>}
            <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
              <Button variant="outline" onClick={() => setCreateOpen(false)}>取消</Button>
              <Button
                disabled={!validName || !readme.trim() || createMutation.isPending}
                onClick={() => createMutation.mutate()}
              >
                {createMutation.isPending ? "创建中…" : "创建"}
              </Button>
            </div>
          </Surface>
        </div>
      )}

      {isLoading ? (
        <div style={{ textAlign: "center", padding: 48 }}>
          <Text color="secondary">loading…</Text>
        </div>
      ) : (domains ?? []).length === 0 ? (
        <Text color="secondary">暂无领域</Text>
      ) : (
        <div style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
          gap: 12,
        }}>
          {(domains ?? []).map((d: Domain) => (
            <Link
              key={d.domain}
              to="/domain/$domain"
              params={{ domain: d.domain }}
              style={{ display: "block", height: "100%", textDecoration: "none" }}
            >
              <Surface style={{
                padding: "20px 24px",
                minHeight: 154,
                height: "100%",
                display: "flex",
                flexDirection: "column",
                cursor: "pointer",
                transition: "border-color 0.15s",
              }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
                  <Text weight="medium" style={{ overflowWrap: "anywhere" }}>{d.domain}</Text>
                  {d.visibility === "private" && <Badge variant="warning">private</Badge>}
                </div>
                <Text
                  size="sm"
                  color="secondary"
                  style={{ marginTop: 4, fontVariantNumeric: "tabular-nums" }}
                >
                  {d.concept_count.toLocaleString("zh-CN")} concepts
                </Text>
                <div style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: "8px 16px",
                  marginTop: "auto",
                  paddingTop: 14,
                  borderTop: "1px solid rgba(128, 128, 128, 0.18)",
                  flexWrap: "wrap",
                  fontVariantNumeric: "tabular-nums",
                }}>
                  <time
                    dateTime={d.updated_at}
                    title={`最后更新：${formatDateTime(d.updated_at)}`}
                    style={{ display: "inline-flex", alignItems: "center", gap: 7 }}
                  >
                    <span
                      aria-hidden="true"
                      style={{ width: 6, height: 6, borderRadius: "50%", background: "#2f8f6b", flex: "0 0 auto" }}
                    />
                    <Text size="xs" weight="medium">{formatRelativeTime(d.updated_at)}更新</Text>
                  </time>
                  <time
                    dateTime={d.created_at}
                    title={`创建时间：${formatDateTime(d.created_at)}`}
                  >
                    <Text size="xs" color="secondary">创建于 {formatCompactDate(d.created_at)}</Text>
                  </time>
                </div>
              </Surface>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
