import {
  createContext,
  useContext,
  useEffect,
  useMemo,
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

export interface RuntimeState {
  status: BootStatus;
  client: CohubClient | null;
  context: WorkRuntimeContext | null;
  /** WorkRuntimeApi.getAccessToken() — returns the Cohub work_session JWT. */
  getToken: (() => Promise<string | null>) | null;
  error: string | null;
}

const RuntimeContext = createContext<RuntimeState | null>(null);

function toMessage(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}

export function RuntimeProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<RuntimeState>({
    status: "booting",
    client: null,
    context: null,
    getToken: null,
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

        setState({
          status: "ready",
          client,
          context,
          getToken: workRuntime
            ? () => workRuntime!.getAccessToken()
            : null,
          error: null,
        });
      } catch (e) {
        if (cancelled) return;
        setState({
          status: "error",
          client: null,
          context: null,
          getToken: null,
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

/**
 * Returns a function that fetches the Cohub work token for OKP API calls.
 * Returns null if the runtime hasn't booted or has no work context.
 */
export function useGetToken(): (() => Promise<string | null>) | null {
  return useRuntime().getToken;
}
