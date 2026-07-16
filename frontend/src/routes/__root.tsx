import { Outlet, createRootRoute, Link, useNavigate } from "@tanstack/react-router";
import { Text, Button, Input, Surface } from "@cloudflare/kumo";
import { useRuntime, useCohubUser } from "../lib/runtime";
import { useApi } from "../api/client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

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
        padding: "12px 24px", gap: 12,
      }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 12 }}>
          <Link to="/" style={{ textDecoration: "none" }}>
            <Text size="lg">okp</Text>
          </Link>
          <Text color="secondary" size="sm">open knowledge pool</Text>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <InviteButton />
          <UserBadge />
        </div>
      </nav>
      <main style={{ maxWidth: 960, margin: "0 auto", padding: "32px 24px" }}>
        <Outlet />
      </main>
    </div>
  );
}

function InviteButton() {
  const api = useApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [code, setCode] = useState("");
  const [msg, setMsg] = useState<string | null>(null);

  const acceptMutation = useMutation({
    mutationFn: (inviteCode: string) => api.acceptInvite(inviteCode),
    onSuccess: (res) => {
      setMsg(`已加入 ${res.domain}，角色 ${res.role}`);
      setCode("");
      queryClient.invalidateQueries({ queryKey: ["me"] });
      setOpen(false);
      navigate({ to: "/domain/$domain", params: { domain: res.domain } });
    },
    onError: (err: Error) => {
      setMsg(err.message || "接受邀请失败");
    },
  });

  return (
    <>
      <Button
        variant="outline"
        onClick={() => {
          setMsg(null);
          setOpen(true);
        }}
      >
        邀请
      </Button>

      {open && (
        <div
          role="dialog"
          aria-modal="true"
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.45)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1000,
            padding: 16,
          }}
          onClick={() => setOpen(false)}
        >
          <Surface
            style={{
              width: "min(440px, 100%)",
              padding: 20,
              display: "flex",
              flexDirection: "column",
              gap: 12,
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <Text weight="medium">接受邀请</Text>
            <Text size="sm" color="secondary">
              输入 host 发给你的邀请码。Work 链接固定为本站，无需额外路由。
            </Text>
            <form
              style={{ display: "flex", gap: 8, flexWrap: "wrap" }}
              onSubmit={(e) => {
                e.preventDefault();
                const v = code.trim();
                if (!v) return;
                setMsg(null);
                acceptMutation.mutate(v);
              }}
            >
              <Input
                value={code}
                onChange={(e) => setCode(e.currentTarget.value.toUpperCase())}
                placeholder="OKP-XXXX-XXXX"
                style={{ flex: 1, minWidth: 180, fontFamily: "ui-monospace, monospace" }}
                autoFocus
              />
              <Button type="submit" disabled={acceptMutation.isPending || !code.trim()}>
                {acceptMutation.isPending ? "提交中…" : "接受"}
              </Button>
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                关闭
              </Button>
            </form>
            {msg && <Text size="sm" color="secondary">{msg}</Text>}
          </Surface>
        </div>
      )}
    </>
  );
}

function UserBadge() {
  const cohubUser = useCohubUser();
  const api = useApi();
  const queryClient = useQueryClient();

  const { data: okpMe } = useQuery({
    queryKey: ["me"],
    queryFn: api.me,
    retry: false,
  });

  const profileMutation = useMutation({
    mutationFn: api.updateMyProfile,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });

  // Sync Cohub profile into OKP once when it differs.
  useEffect(() => {
    if (!cohubUser || !okpMe || profileMutation.isPending || profileMutation.isSuccess) return;

    const profile = {
      username: cohubUser.username ?? "",
      display_name: cohubUser.displayName ?? "",
      avatar_url: cohubUser.avatarUrl ?? "",
    };
    if (
      profile.username === (okpMe.username ?? "") &&
      profile.display_name === (okpMe.display_name ?? "") &&
      profile.avatar_url === (okpMe.avatar_url ?? "")
    ) return;

    profileMutation.mutate(profile);
  }, [
    cohubUser?.username,
    cohubUser?.displayName,
    cohubUser?.avatarUrl,
    okpMe?.username,
    okpMe?.display_name,
    okpMe?.avatar_url,
    profileMutation.isPending,
    profileMutation.isSuccess,
  ]);

  if (!cohubUser && !okpMe) {
    return <Text size="sm" color="secondary">未登录</Text>;
  }

  const name = cohubUser?.displayName || cohubUser?.username || okpMe?.display_name || okpMe?.username || okpMe?.uuid?.slice(0, 8) || "user";
  const avatar = cohubUser?.avatarUrl || okpMe?.avatar_url;
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
