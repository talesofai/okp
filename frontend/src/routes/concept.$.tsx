import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { useApi } from "../api/client";
import { Text, Badge, Surface, Empty, Loader } from "@cloudflare/kumo";

declare const marked: { parse: (md: string) => string };

export const Route = createFileRoute("/concept/$")({
  component: ConceptDetail,
});

function ConceptDetail() {
  const api = useApi();
  const params = Route.useParams() as Record<string, string>;
  const id = params.$ ?? params._splat ?? "";
  const bodyRef = useRef<HTMLDivElement>(null);

  const { data: concept, isLoading, error } = useQuery({
    queryKey: ["concept", id],
    queryFn: () => api.getConcept(id),
  });

  useEffect(() => {
    if (concept?.body && bodyRef.current) {
      bodyRef.current.innerHTML = typeof marked !== "undefined"
        ? marked.parse(concept.body)
        : `<pre>${concept.body}</pre>`;
    }
  }, [concept]);

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
      </div>

      {concept.resource && (
        <Surface style={{ padding: "12px 16px", marginTop: 24 }}>
          <a href={concept.resource} target="_blank" rel="noopener" style={{ textDecoration: "none" }}>
            <Text size="sm">📎 资源链接 →</Text>
          </a>
        </Surface>
      )}

      {concept.body && (
        <div
          ref={bodyRef}
          className="markdown-body"
          style={{ marginTop: 32, lineHeight: 1.7 }}
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
    </div>
  );
}
