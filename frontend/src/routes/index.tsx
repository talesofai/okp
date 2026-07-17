import { createFileRoute, useNavigate, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useApi, type Domain } from "../api/client";
import { Input, Text, Surface } from "@cloudflare/kumo";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const api = useApi();
  const navigate = useNavigate();

  const { data: domains, isLoading } = useQuery({
    queryKey: ["domains"],
    queryFn: api.domains,
  });

  return (
    <div>
      <div style={{ textAlign: "center", marginBottom: 40 }}>
        <Text size="2xl" weight="bold" as="h1" style={{ marginBottom: 8 }}>Open Knowledge Protocol</Text>
        <Text color="secondary">Browse domains, search concepts</Text>

        <form
          style={{ marginTop: 24 }}
          onSubmit={(e) => {
            e.preventDefault();
            const q = (e.currentTarget.elements.namedItem("q") as HTMLInputElement).value;
            if (q.trim()) {
              navigate({ to: "/search", search: { q: q.trim() } });
            }
          }}
        >
          <Input name="q" placeholder="搜索概念…" style={{ width: 360 }} />
        </form>
      </div>

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
            <Link key={d.domain} to="/domain/$domain" params={{ domain: d.domain }} style={{ textDecoration: "none" }}>
              <Surface style={{
                padding: "20px 24px",
                cursor: "pointer",
                transition: "border-color 0.15s",
              }}>
                <Text weight="medium">{d.domain}</Text>
                <Text size="sm" color="secondary" style={{ marginTop: 4 }}>
                  {d.concept_count} concepts
                </Text>
              </Surface>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
