import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { useApi } from "../api/client";
import { Text, Badge, Surface, Empty, Loader, Button } from "@cloudflare/kumo";
import { formatDateTime } from "../lib/time";

declare const marked: { parse: (md: string) => string };

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

export const Route = createFileRoute("/concept/$")({
  component: ConceptDetail,
});

function ConceptDetail() {
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const params = Route.useParams() as Record<string, string>;
  const id = params.$ ?? params._splat ?? "";

  const { data: concept, isLoading, error } = useQuery({
    queryKey: ["concept", id],
    queryFn: () => api.getConcept(id),
  });

  const { data: me } = useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    retry: false,
  });

  const { data: meta, isFetched: metaFetched } = useQuery({
    queryKey: ["domain-meta", concept?.domain],
    queryFn: () => api.getDomainReadme(concept!.domain),
    enabled: Boolean(concept?.domain),
    retry: false,
  });

  const membership = me?.domains?.find((d) => d.domain === concept?.domain)?.role;
  const canDelete = membership === "host" || membership === "writer" ||
    (me?.role === "admin" && metaFetched && (meta?.visibility ?? "public") === "public");

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteConcept(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["domain", concept?.domain] });
      await queryClient.invalidateQueries({ queryKey: ["domains"] });
      navigate({ to: "/domain/$domain", params: { domain: concept!.domain } });
    },
  });

  const bodyHtml = useMemo(() => {
    if (!concept?.body) return "";
    return typeof marked !== "undefined"
      ? marked.parse(concept.body)
      : `<pre>${concept.body}</pre>`;
  }, [concept?.body]);

  if (isLoading) {
    return <div style={{ textAlign: "center", padding: 64 }}><Loader /></div>;
  }
  if (error) {
    return <Empty title="加载失败" description={(error as Error).message} />;
  }
  if (!concept) {
    return <Empty title="概念不存在" />;
  }

  return (
    <div style={{ maxWidth: 760 }}>
      <style dangerouslySetInnerHTML={{ __html: markdownStyles }} />

      <Link to="/domain/$domain" params={{ domain: concept.domain }} style={{ textDecoration: "none" }}>
        <Text size="sm" color="secondary">← {concept.domain}</Text>
      </Link>

      <div style={{ marginTop: 16 }}>
        <Text size="xs" color="secondary">
          {concept.domain} / {concept.type}
        </Text>
        <Text size="2xl" weight="bold" as="h1" style={{ marginTop: 4 }}>
          {concept.title}
        </Text>

        {concept.description && (
          <Text color="secondary" style={{ marginTop: 8 }}>
            {concept.description}
          </Text>
        )}

        <div style={{ display: "flex", gap: 6, marginTop: 16, flexWrap: "wrap", alignItems: "center" }}>
          {concept.tags?.map((t: string) => (
            <Badge key={t} variant="default">{t}</Badge>
          ))}
          <Badge variant={concept.status === "accepted" ? "success" : "warning"}>
            {concept.status}
          </Badge>
        </div>
        <div style={{ display: "flex", gap: 14, marginTop: 12, flexWrap: "wrap" }}>
          <Text size="xs" color="secondary">创建 {formatDateTime(concept.created_at)}</Text>
          <Text size="xs" color="secondary">更新 {formatDateTime(concept.updated_at)}</Text>
        </div>
      </div>

      {concept.resource && (
        <Surface style={{ padding: "12px 16px", marginTop: 24 }}>
          <a href={concept.resource} target="_blank" rel="noopener" style={{ textDecoration: "none" }}>
            <Text size="sm">📎 资源链接 →</Text>
          </a>
        </Surface>
      )}

      {bodyHtml && (
        <div
          className="okp-md"
          style={{ marginTop: 32, lineHeight: 1.7, fontSize: 15 }}
          dangerouslySetInnerHTML={{ __html: bodyHtml }}
        />
      )}

      {concept.provenance && Object.keys(concept.provenance).length > 0 && (
        <Surface style={{ padding: "16px 20px", marginTop: 40 }}>
          <details>
            <summary style={{ cursor: "pointer" }}>
              <Text size="sm" weight="medium">来源信息</Text>
            </summary>
            <pre style={{ marginTop: 12, fontSize: 13, overflow: "auto" }}>
              {JSON.stringify(concept.provenance, null, 2)}
            </pre>
          </details>
        </Surface>
      )}

      {canDelete && (
        <div style={{ borderTop: "1px solid rgba(128,128,128,0.22)", marginTop: 40, paddingTop: 24 }}>
          <div style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "center", flexWrap: "wrap" }}>
            <div>
              <Text weight="medium" color="danger">删除 Concept</Text>
              <Text size="sm" color="secondary">同时删除该 concept 的 revision 历史和所有相关 links。</Text>
            </div>
            <Button
              variant="outline"
              disabled={deleteMutation.isPending}
              style={{ color: "#dc2626", borderColor: "rgba(220,38,38,0.45)" }}
              onClick={() => {
                if (window.confirm(`永久删除 concept ${concept.id}？`)) deleteMutation.mutate();
              }}
            >
              {deleteMutation.isPending ? "删除中…" : "删除"}
            </Button>
          </div>
          {deleteMutation.error && (
            <Text size="sm" color="danger" style={{ marginTop: 8 }}>{(deleteMutation.error as Error).message}</Text>
          )}
        </div>
      )}
    </div>
  );
}
