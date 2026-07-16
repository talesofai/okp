import { useMemo } from "react";
import { useRuntime } from "../lib/runtime";

const OKP_BASE = "https://okp.neta.art";

export interface Domain {
  domain: string;
  concept_count: number;
}

export interface Concept {
  id: string;
  domain: string;
  type: string;
  title: string;
  description: string;
  tags: string[];
  body?: string;
  resource?: string;
  status: string;
  frontmatter?: Record<string, string>;
  provenance?: Record<string, string>;
  content_hash?: string;
  created_at?: string;
  updated_at?: string;
}

export interface MeResponse {
  uuid: string;
  username?: string;
  display_name?: string;
  avatar_url?: string;
  email?: string;
  role?: string;
}

async function fetchOkp<T>(
  path: string,
  getToken: (() => Promise<string | null>) | null,
  init?: RequestInit,
): Promise<T> {
  const token = getToken ? await getToken() : null;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
  const res = await fetch(`${OKP_BASE}${path}`, { ...init, headers });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export function useApi() {
  const { getToken } = useRuntime();

  return useMemo(
    () => ({
      domains: () => fetchOkp<Domain[]>("/api/v1/domains", getToken),

      search: (params: {
        q?: string;
        domain?: string;
        type?: string;
        tag?: string;
        limit?: number;
        offset?: number;
      }) => {
        const sp = new URLSearchParams();
        if (params.q) sp.set("q", params.q);
        if (params.domain) sp.set("domain", params.domain);
        if (params.type) sp.set("type", params.type);
        if (params.tag) sp.set("tag", params.tag);
        sp.set("limit", String(params.limit ?? 50));
        if (params.offset) sp.set("offset", String(params.offset));
        sp.set("status", "accepted");
        return fetchOkp<Concept[]>(`/api/v1/concepts?${sp}`, getToken);
      },

      getConcept: (id: string) =>
        fetchOkp<Concept>(
          `/api/v1/concepts/${encodeURIComponent(id)}`,
          getToken,
        ),

      getDomainReadme: (domain: string) =>
        fetchOkp<{ domain: string; readme: string; schema: string }>(
          `/api/v1/domains/${encodeURIComponent(domain)}/readme`,
          getToken,
        ),

      getLinks: (id: string) =>
        fetchOkp<{ outgoing: unknown[]; backlinks: unknown[] }>(
          `/api/v1/links/${encodeURIComponent(id)}`,
          getToken,
        ),
    }),
    [getToken],
  );
}
