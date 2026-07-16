import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  createCohubClient,
  createWorkRuntime,
  resolveWorkTransport,
  type WorkRuntimeApi,
  type WorkRuntimeContext,
} from "@neta-art/cohub";

export type BootStatus = "booting" | "ready" | "error";

type CohubClient = ReturnType<typeof createCohubClient>;

export interface CohubUser {
  uuid: string;
  username: string | null;
  displayName: string | null;
  avatarUrl: string | null;
  email: string | null;
}

export interface RuntimeState {
  status: BootStatus;
  client: CohubClient | null;
  context: WorkRuntimeContext | null;
  /** WorkRuntimeApi.getAccessToken() — returns the Cohub work_session JWT. */
  getToken: (() => Promise<string | null>) | null;
  /** Cohub user profile from client.user.getMe(). */
  user: CohubUser | null;
  error: string | null;
}

const RuntimeContext = createContext<RuntimeState | null>(null);

function toMessage(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}

/** Check if a JWT is expired (with 30s leeway). */
function isTokenExpired(token: string): boolean {
  const parts = token.split(".");
  if (parts.length < 2) return true;
  try {
    const payload = JSON.parse(
      atob(parts[1].replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - (parts[1].length % 4)) % 4)),
    );
    if (!payload.exp) return false; // no exp, assume valid
    return payload.exp * 1000 <= Date.now() + 30_000;
  } catch {
    return true;
  }
}

export function RuntimeProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<RuntimeState>({
    status: "booting",
    client: null,
    context: null,
    getToken: null,
    user: null,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const client = createCohubClient();
        const context = await client.context();
        if (cancelled) return;

        const workId = context?.work?.id ?? null;
        let workRuntime: WorkRuntimeApi | null = null;
        if (workId) {
          const transport = resolveWorkTransport(
            window.parent !== window
              ? undefined
              : { workId, brokerOrigin: "https://cohub.run", mode: "broker" },
          );
          workRuntime = createWorkRuntime(transport, workId);
        }

        let user: CohubUser | null = null;
        try {
          const me = await client.user.getMe();
          user = {
            uuid: me.uuid,
            username: me.profile?.username ?? null,
            displayName: me.profile?.displayName ?? null,
            avatarUrl: me.profile?.avatarUrl ?? null,
            email: me.email ?? null,
          };
        } catch {
          // getMe may fail outside Work runtime; that's OK
        }

        if (cancelled) return;
        setState({
          status: "ready",
          client,
          context,
          getToken: workRuntime
            ? async () => {
              const t = await workRuntime!.getAccessToken();
              if (t && !isTokenExpired(t)) return t;
              // Token expired or missing — force refresh
              return workRuntime!.getAccessToken({ forceRefresh: true });
            }
            : null,
          user,
          error: null,
        });
      } catch (e) {
        if (cancelled) return;
        setState({
          status: "error",
          client: null,
          context: null,
          getToken: null,
          user: null,
          error: toMessage(e),
        });
      }
    })();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <RuntimeContext.Provider value={state}>
      {children}
    </RuntimeContext.Provider>
  );
}

export function useRuntime(): RuntimeState {
  const v = useContext(RuntimeContext);
  if (!v) throw new Error("useRuntime must be used within RuntimeProvider");
  return v;
}

export function useWorkContext(): WorkRuntimeContext | null {
  return useRuntime().context;
}

export function useCohubUser(): CohubUser | null {
  return useRuntime().user;
}
