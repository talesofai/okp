import { createFileRoute, useNavigate, Link } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useApi, type Domain } from "../api/client";
import { Input, Text, Surface, Button } from "@cloudflare/kumo";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [inviteCode, setInviteCode] = useState("");
  const [inviteMsg, setInviteMsg] = useState<string | null>(null);

  const { data: domains, isLoading } = useQuery({
    queryKey: ["domains"],
    queryFn: api.domains,
  });

  const acceptMutation = useMutation({
    mutationFn: (code: string) => api.acceptInvite(code),
    onSuccess: (res) => {
      setInviteMsg(`已加入 ${res.domain}，角色 ${res.role}`);
      setInviteCode("");
      queryClient.invalidateQueries({ queryKey: ["me"] });
      navigate({ to: "/domain/$domain", params: { domain: res.domain } });
    },
    onError: (err: Error) => {
      setInviteMsg(err.message || "接受邀请失败");
    },
  });

  return (
    <div>
      <div style={{ textAlign: "center", marginBottom: 40 }}>
        <Text size="2xl" weight="bold" as="h1" style={{ marginBottom: 8 }}>知识池</Text>
        <Text color="secondary">浏览所有知识领域，搜索概念</Text>

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

      <Surface style={{
        padding: 20,
        marginBottom: 28,
        borderLeft: "3px solid #6366f1",
      }}>
        <Text weight="medium" style={{ marginBottom: 8 }}>接受邀请</Text>
        <Text size="sm" color="secondary" style={{ marginBottom: 12 }}>
          收到邀请码后，在此输入并确认。Work 链接固定为当前页面。
        </Text>
        <form
          style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}
          onSubmit={(e) => {
            e.preventDefault();
            const code = inviteCode.trim();
            if (!code) return;
            setInviteMsg(null);
            acceptMutation.mutate(code);
          }}
        >
          <Input
            value={inviteCode}
            onChange={(e) => setInviteCode(e.currentTarget.value.toUpperCase())}
            placeholder="OKP-XXXX-XXXX"
            style={{ width: 220, fontFamily: "ui-monospace, monospace" }}
          />
          <Button
            type="submit"
            disabled={acceptMutation.isPending || !inviteCode.trim()}
          >
            {acceptMutation.isPending ? "提交中…" : "接受邀请"}
          </Button>
        </form>
        {inviteMsg && (
          <Text size="sm" style={{ marginTop: 10 }} color="secondary">
            {inviteMsg}
          </Text>
        )}
      </Surface>

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
