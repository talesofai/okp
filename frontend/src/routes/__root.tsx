import { Outlet, createRootRoute, Link } from "@tanstack/react-router";
import { Text } from "@cloudflare/kumo";
import { useRuntime } from "../lib/runtime";

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
        display: "flex", alignItems: "baseline", gap: 12,
        padding: "16px 24px",
      }}>
        <Link to="/" style={{ textDecoration: "none" }}>
          <Text size="lg" weight="bold">okp</Text>
        </Link>
        <Text color="secondary" size="sm">open knowledge pool</Text>
      </nav>
      <main style={{ maxWidth: 960, margin: "0 auto", padding: "32px 24px" }}>
        <Outlet />
      </main>
    </div>
  );
}
