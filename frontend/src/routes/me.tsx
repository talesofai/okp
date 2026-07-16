import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../api/client";
import { useCohubUser } from "../lib/runtime";
import { Text, Surface } from "@cloudflare/kumo";

export const Route = createFileRoute("/me")({
  component: MePage,
});

function MePage() {
  const api = useApi();
  const cohubUser = useCohubUser();

  const { data: okpMe, isLoading } = useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    retry: false,
  });

  if (isLoading) {
    return <Text color="secondary">loading…</Text>;
  }

  return (
    <div style={{ maxWidth: 560 }}>
      <Link to="/" style={{ textDecoration: "none", marginBottom: 16, display: "inline-block" }}>
        <Text size="sm" color="secondary">← 返回</Text>
      </Link>

      <Text size="xl" weight="bold" as="h1" style={{ marginBottom: 24 }}>我的资料</Text>

      {cohubUser && (
        <Surface style={{ padding: 24, marginBottom: 16 }}>
          <Text size="sm" color="secondary" style={{ marginBottom: 12 }}>Cohub 账号</Text>
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            {cohubUser.avatarUrl && (
              <img
                src={cohubUser.avatarUrl}
                alt={cohubUser.displayName ?? cohubUser.username ?? "avatar"}
                style={{ width: 56, height: 56, borderRadius: "50%", objectFit: "cover" }}
              />
            )}
            <div>
              <Text weight="medium">{cohubUser.displayName || cohubUser.username || "未知"}</Text>
              {cohubUser.username && cohubUser.displayName && (
                <Text size="sm" color="secondary">@{cohubUser.username}</Text>
              )}
              {cohubUser.email && (
                <Text size="sm" color="secondary">{cohubUser.email}</Text>
              )}
            </div>
          </div>
        </Surface>
      )}

      {okpMe && (
        <Surface style={{ padding: 24, marginBottom: 16 }}>
          <Text size="sm" color="secondary" style={{ marginBottom: 12 }}>OKP 账号</Text>
          <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "8px 16px", fontSize: 14 }}>
            <Text size="sm" color="secondary">UUID</Text>
            <Text size="sm" style={{ fontFamily: "monospace", wordBreak: "break-all" }}>{okpMe.uuid}</Text>

            <Text size="sm" color="secondary">角色</Text>
            <span style={{
              display: "inline-block",
              fontSize: 12, padding: "2px 8px", borderRadius: 4,
              background: okpMe.role === "writer" ? "#10b981" : "#6b7280",
              color: "#fff",
            }}>
              {okpMe.role || "reader"}
            </span>

            <Text size="sm" color="secondary">认证方式</Text>
            <Text size="sm">{(okpMe as Record<string, unknown>).auth_type as string || "—"}</Text>

            <Text size="sm" color="secondary">首次访问</Text>
            <Text size="sm">{(okpMe as Record<string, unknown>).created_at as string || "—"}</Text>

            <Text size="sm" color="secondary">最近活跃</Text>
            <Text size="sm">{(okpMe as Record<string, unknown>).last_seen as string || "—"}</Text>
          </div>
        </Surface>
      )}

      <Surface style={{ padding: 24, marginTop: 16 }}>
        <Text size="sm" color="secondary">权限说明</Text>
        <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: "#10b981", color: "#fff" }}>writer</span>
            <Text size="sm">可以写入概念、创建领域</Text>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: "#6b7280", color: "#fff" }}>reader</span>
            <Text size="sm">只读，可浏览和搜索所有公开领域</Text>
          </div>
          <Text size="sm" color="secondary" style={{ marginTop: 8 }}>
            需要写入权限请联系管理员将角色设为 writer。
          </Text>
        </div>
      </Surface>
    </div>
  );
}
