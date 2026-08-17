import { useMemo } from "react";
import { useRuntime } from "../lib/runtime";

const OKP_BASE = "https://okp.neta.art";
export const OKP_WORK_URL = "https://cohub.run/koujiaxin/real-canvas/w/okp";

export interface Domain {
  domain: string;
  concept_count: number;
  has_readme?: boolean;
  visibility: "public" | "private";
  created_at?: string;
  updated_at?: string;
}

export interface DomainMeta {
  domain: string;
  readme: string;
  schema: Record<string, unknown>;
  visibility: "public" | "private";
  created_at?: string;
  updated_at?: string;
}

export interface DomainActivityPoint {
  date: string;
  created: number;
  updated: number;
  activity: number;
}

export interface DomainActivity {
  domain: string;
  days: number;
  from: string;
  to: string;
  points: DomainActivityPoint[];
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

export interface DomainMembership {
  domain: string;
  user_id: string;
  role: string;
  username?: string;
  display_name?: string;
  avatar_url?: string;
  created_at?: string;
  updated_at?: string;
}

export interface MeResponse {
  uuid: string;
  auth_type?: string;
  role?: string;
  username?: string;
  display_name?: string;
  avatar_url?: string;
  last_seen?: string;
  created_at?: string;
  domains?: DomainMembership[];
}

export interface CreateInviteResponse {
  id: string;
  domain: string;
  role: string;
  created_by: string;
  created_at: string;
  expires_at: string;
  max_uses: number;
  used_count: number;
  code: string;
  work_url: string;
  share_text: string;
}

export interface AcceptInviteResponse {
  status: string;
  domain: string;
  role: string;
  member: DomainMembership;
}

export interface DomainInviteRow {
  id: string;
  domain: string;
  role: string;
  created_by: string;
  created_at: string;
  expires_at: string;
  max_uses: number;
  used_count: number;
  revoked_at?: string | null;
  last_used_at?: string | null;
  last_used_by?: string;
  status: string;
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

async function fetchOkpVoid(
  path: string,
  getToken: (() => Promise<string | null>) | null,
  init: RequestInit,
): Promise<void> {
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
}

export function useApi() {
  const { getToken } = useRuntime();

  return useMemo(
    () => ({
      me: () => fetchOkp<MeResponse>("/api/v1/me", getToken),

      updateMyProfile: (profile: {
        username: string;
        display_name: string;
        avatar_url: string;
      }) => fetchOkp<{ status: string }>("/api/v1/me/profile", getToken, {
        method: "PUT",
        body: JSON.stringify(profile),
      }),

      domains: () => fetchOkp<Domain[]>("/api/v1/domains", getToken),

      putDomain: (domain: string, body: { readme: string; visibility: "public" | "private" }) =>
        fetchOkp<DomainMeta>(
          `/api/v1/domains/${encodeURIComponent(domain)}`,
          getToken,
          { method: "PUT", body: JSON.stringify(body) },
        ),

      deleteDomain: (domain: string) =>
        fetchOkpVoid(
          `/api/v1/domains/${encodeURIComponent(domain)}`,
          getToken,
          { method: "DELETE" },
        ),

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
          `/api/v1/concepts/${id.split("/").map(encodeURIComponent).join("/")}`,
          getToken,
        ),

      deleteConcept: (id: string) =>
        fetchOkpVoid(
          `/api/v1/concepts/${id.split("/").map(encodeURIComponent).join("/")}`,
          getToken,
          { method: "DELETE" },
        ),

      getDomainReadme: (domain: string) =>
        fetchOkp<DomainMeta>(
          `/api/v1/domains/${encodeURIComponent(domain)}`,
          getToken,
        ),

      getDomainActivity: (domain: string, days = 30) =>
        fetchOkp<DomainActivity>(
          `/api/v1/domains/${encodeURIComponent(domain)}/activity?days=${days}`,
          getToken,
        ),

      getLinks: (id: string) =>
        fetchOkp<{ outgoing: unknown[]; backlinks: unknown[] }>(
          `/api/v1/links/${id.split("/").map(encodeURIComponent).join("/")}`,
          getToken,
        ),

      createInvite: (domain: string, body?: { role?: string; expires_in_hours?: number; max_uses?: number }) =>
        fetchOkp<CreateInviteResponse>(
          `/api/v1/domains/${encodeURIComponent(domain)}/invites`,
          getToken,
          {
            method: "POST",
            body: JSON.stringify({
              role: body?.role ?? "writer",
              expires_in_hours: body?.expires_in_hours ?? 72,
              max_uses: body?.max_uses ?? 1,
            }),
          },
        ),

      listInvites: (domain: string) =>
        fetchOkp<DomainInviteRow[]>(
          `/api/v1/domains/${encodeURIComponent(domain)}/invites`,
          getToken,
        ),

      revokeInvite: (domain: string, id: string) =>
        fetchOkp<{ status: string; id: string }>(
          `/api/v1/domains/${encodeURIComponent(domain)}/invites/${encodeURIComponent(id)}`,
          getToken,
          { method: "DELETE" },
        ),

      acceptInvite: (code: string) =>
        fetchOkp<AcceptInviteResponse>(
          `/api/v1/invites/accept`,
          getToken,
          {
            method: "POST",
            body: JSON.stringify({ code }),
          },
        ),

      listMembers: (domain: string) =>
        fetchOkp<DomainMembership[]>(
          `/api/v1/domains/${encodeURIComponent(domain)}/members`,
          getToken,
        ),
    }),
    [getToken],
  );
}
