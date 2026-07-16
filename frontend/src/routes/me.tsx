import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../api/client";
import { useCohubUser } from "../lib/runtime";
import { Text, Surface } from "@cloudflare/kumo";

export const Route = createFileRoute("/me")({
  component: MePage,
});

function roleColor(role?: string) {
  switch (role) {
    case "admin":
      return "#7c3aed";
    case "host":
      return "#2563eb";
    case "writer":
      return "#10b981";
    default:
      return "#6b7280";
  }
}

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

  const globalRole = okpMe?.role || "reader";

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

            <Text size="sm" color="secondary">全局角色</Text>
            <span style={{
              display: "inline-block",
              fontSize: 12, padding: "2px 8px", borderRadius: 4,
              background: roleColor(globalRole),
              color: "#fff",
              width: "fit-content",
            }}>
              {globalRole}
            </span>

            <Text size="sm" color="secondary">认证方式</Text>
            <Text size="sm">{okpMe.auth_type || "—"}</Text>

            <Text size="sm" color="secondary">首次访问</Text>
            <Text size="sm">{okpMe.created_at || "—"}</Text>

            <Text size="sm" color="secondary">最近活跃</Text>
            <Text size="sm">{okpMe.last_seen || "—"}</Text>
          </div>
        </Surface>
      )}

      <Surface style={{ padding: 24, marginBottom: 16 }}>
        <Text size="sm" color="secondary" style={{ marginBottom: 12 }}>我的 Domains</Text>
        {(okpMe?.domains ?? []).length === 0 ? (
          <Text size="sm" color="secondary">暂无显式 domain 成员身份（公开 domain 默认可读）</Text>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {(okpMe?.domains ?? []).map((d) => (
              <div key={`${d.domain}-${d.role}`} style={{ display: "flex", justifyContent: "space-between", gap: 12 }}>
                <Link to="/domain/$domain" params={{ domain: d.domain }} style={{ textDecoration: "none" }}>
                  <Text size="sm">{d.domain}</Text>
                </Link>
                <span style={{
                  fontSize: 11, padding: "2px 8px", borderRadius: 4,
                  background: roleColor(d.role), color: "#fff",
                }}>
                  {d.role}
                </span>
              </div>
            ))}
          </div>
        )}
      </Surface>

      <Surface style={{ padding: 24, marginTop: 16 }}>
        <Text size="sm" color="secondary">权限说明</Text>
        <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: roleColor("admin"), color: "#fff" }}>admin</span>
            <Text size="sm">全局管理员，可写任何 domain</Text>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: roleColor("host"), color: "#fff" }}>host</span>
            <Text size="sm">管理 domain README/成员/邀请，可写 concept</Text>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: roleColor("writer"), color: "#fff" }}>writer</span>
            <Text size="sm">可在授权 domain 写入概念</Text>
          </div>
          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
            <span style={{ fontSize: 12, padding: "2px 8px", borderRadius: 4, background: roleColor("reader"), color: "#fff" }}>reader</span>
            <Text size="sm">默认公开只读；私有 domain 需显式授权</Text>
          </div>
          <Text size="sm" color="secondary" style={{ marginTop: 8 }}>
            需要写入权限时，让领域管理员生成邀请码，再点右上角「邀请」输入。
          </Text>
        </div>
      </Surface>
    </div>
  );
}
