import { Outlet, createRootRoute, Link } from "@tanstack/react-router";
import { Text } from "@cloudflare/kumo";
import { useRuntime, useCohubUser } from "../lib/runtime";
import { useApi } from "../api/client";
import { useQuery } from "@tanstack/react-query";

export const Route = createRootRoute({
  component: Root,
});

function Root() {
  const { status, error } = useRuntime();

  if (status === "booting") {
    return (
      <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <Text color="secondary">loading…</Text>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <Text color="danger">failed to boot: {error}</Text>
      </div>
    );
  }

  return (
    <div style={{ minHeight: "100vh" }}>
      <nav className="bg-kumo-surface-elevated border-b border-kumo-border" style={{
        display: "flex", alignItems: "center", justifyContent: "space-between",
        padding: "12px 24px",
      }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 12 }}>
          <Link to="/" style={{ textDecoration: "none" }}>
            <Text size="lg">okp</Text>
          </Link>
          <Text color="secondary" size="sm">open knowledge pool</Text>
        </div>
        <UserBadge />
      </nav>
      <main style={{ maxWidth: 960, margin: "0 auto", padding: "32px 24px" }}>
        <Outlet />
      </main>
    </div>
  );
}

function UserBadge() {
  const cohubUser = useCohubUser();
  const api = useApi();

  const { data: okpMe } = useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    retry: false,
  });

  if (!cohubUser && !okpMe) {
    return <Text size="sm" color="secondary">未登录</Text>;
  }

  const name = cohubUser?.displayName || cohubUser?.username || okpMe?.uuid?.slice(0, 8) || "user";
  const avatar = cohubUser?.avatarUrl;
  const role = okpMe?.role || "reader";
  const roleBg =
    role === "admin" ? "#7c3aed" :
    role === "host" ? "#2563eb" :
    role === "writer" ? "#10b981" : "#6b7280";

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      <Link to="/me" style={{ textDecoration: "none", display: "flex", alignItems: "center", gap: 8 }}>
        {avatar ? (
          <img
            src={avatar}
            alt={name}
            style={{ width: 28, height: 28, borderRadius: "50%", objectFit: "cover" }}
          />
        ) : (
          <div style={{
            width: 28, height: 28, borderRadius: "50%",
            background: "#6366f1", color: "#fff",
            display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 12,
          }}>
            {name[0]?.toUpperCase()}
          </div>
        )}
        <Text size="sm">{name}</Text>
      </Link>
      <span style={{
        fontSize: 10, padding: "2px 8px", borderRadius: 4,
        background: roleBg,
        color: "#fff",
      }}>
        {role}
      </span>
    </div>
  );
}
