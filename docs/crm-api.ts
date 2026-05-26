/**
 * CRM API client for Next.js (App Router / RSC / client components).
 * Always call through the gateway — never the service port in the browser.
 *
 *   NEXT_PUBLIC_CRM_API_URL=http://localhost:8080/api/v1/crm/v1
 */

export type Paginated<T> = {
  data: T[];
  meta: { total: number; limit: number; offset: number };
};

export type CrmFetchOptions = RequestInit & {
  token?: string;
};

function baseUrl(): string {
  const url = process.env.NEXT_PUBLIC_CRM_API_URL ?? "http://localhost:8080/api/v1/crm/v1";
  return url.replace(/\/$/, "");
}

export async function crmFetch<T>(path: string, opts: CrmFetchOptions = {}): Promise<T> {
  const { token, headers, ...rest } = opts;
  const res = await fetch(`${baseUrl()}${path.startsWith("/") ? path : `/${path}`}`, {
    ...rest,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...headers,
    },
    credentials: "include",
  });
  if (!res.ok) {
    throw new Error(`CRM ${res.status}: ${await res.text()}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

function qs(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

/** Factory for typed list/create helpers used across CRM pages. */
function listResource<T>(path: string) {
  return (token?: string, params?: Record<string, string | number | undefined>) =>
    crmFetch<Paginated<T>>(`${path}${qs(params ?? {})}`, { token });
}

function createResource<T>(path: string) {
  return (body: unknown, token?: string) =>
    crmFetch<T>(path, { method: "POST", body: JSON.stringify(body), token });
}

export const crmApi = {
  // Platform / layout
  bootstrap: (token?: string) => crmFetch("/bootstrap", { token }),
  session: (token?: string) => crmFetch("/auth/session", { token }),
  permissionsCatalog: (token?: string) => crmFetch("/permissions/catalog", { token }),
  permissionsBuiltin: (token?: string) => crmFetch("/permissions/builtin", { token }),
  permissionsCheck: (keys: string[], token?: string) =>
    crmFetch("/permissions/check", { method: "POST", body: JSON.stringify({ keys }), token }),
  permissionsMe: (token?: string) => crmFetch("/permissions/me", { token }),
  platformStatus: (token?: string) => crmFetch("/platform/status", { token }),
  notifications: (token?: string) => crmFetch<{ data: unknown[] }>("/notifications", { token }),
  lookups: (kind: string, token?: string) =>
    crmFetch<{ data: unknown[] }>(`/lookups/${encodeURIComponent(kind)}`, { token }),

  // Command tower
  overview: (range = "week", token?: string) =>
    crmFetch(`/overview${qs({ range })}`, { token }),
  pipelineBoard: (token?: string, owner?: string) =>
    crmFetch(`/pipeline/board${qs({ owner })}`, { token }),
  dealForecast: (token?: string, quarter = "Q2") =>
    crmFetch(`/deals/forecast${qs({ quarter })}`, { token }),

  // Sales core
  accounts: listResource<unknown>("/accounts"),
  createAccount: createResource<unknown>("/accounts"),
  getAccount: (id: string, token?: string) => crmFetch(`/accounts/${id}`, { token }),
  account360: (id: string, token?: string) => crmFetch(`/accounts/${id}/360`, { token }),
  patchAccount: (id: string, body: unknown, token?: string) =>
    crmFetch(`/accounts/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),
  deleteAccount: (id: string, token?: string) =>
    crmFetch<void>(`/accounts/${id}`, { method: "DELETE", token }),

  contacts: listResource<unknown>("/contacts"),
  createContact: createResource<unknown>("/contacts"),
  getContact: (id: string, token?: string) => crmFetch(`/contacts/${id}`, { token }),

  leads: listResource<unknown>("/leads"),
  createLead: createResource<unknown>("/leads"),
  getLead: (id: string, token?: string) => crmFetch(`/leads/${id}`, { token }),
  patchLead: (id: string, body: unknown, token?: string) =>
    crmFetch(`/leads/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),
  convertLead: (id: string, body: unknown, token?: string) =>
    crmFetch(`/leads/${id}/convert`, { method: "POST", body: JSON.stringify(body), token }),

  deals: listResource<unknown>("/deals"),
  createDeal: createResource<unknown>("/deals"),
  getDeal: (id: string, token?: string) => crmFetch(`/deals/${id}`, { token }),
  patchDeal: (id: string, body: unknown, token?: string) =>
    crmFetch(`/deals/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),
  setDealStage: (id: string, stage: string, token?: string) =>
    crmFetch(`/deals/${id}/stage`, { method: "PATCH", body: JSON.stringify({ stage }), token }),

  setDealStage: (id: string, stage: string, token?: string) =>
    crmFetch(`/deals/${id}/stage`, { method: "PATCH", body: JSON.stringify({ stage }), token }),
  markDealWon: (id: string, token?: string) =>
    crmFetch(`/deals/${id}/won`, { method: "POST", body: "{}", token }),

  patchContact: (id: string, body: unknown, token?: string) =>
    crmFetch(`/contacts/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),
  deleteContact: (id: string, token?: string) =>
    crmFetch<void>(`/contacts/${id}`, { method: "DELETE", token }),

  quotes: listResource<unknown>("/quotes"),
  createQuote: createResource<unknown>("/quotes"),
  getQuote: (id: string, token?: string) => crmFetch(`/quotes/${id}`, { token }),
  patchQuote: (id: string, body: unknown, token?: string) =>
    crmFetch(`/quotes/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),
  sendQuote: (id: string, token?: string) =>
    crmFetch(`/quotes/${id}/send`, { method: "POST", body: "{}", token }),
  signQuote: (id: string, token?: string) =>
    crmFetch(`/quotes/${id}/sign`, { method: "POST", body: "{}", token }),

  activities: listResource<unknown>("/activities"),
  createActivity: createResource<unknown>("/activities"),
  activitiesStreamUrl: (token?: string) => {
    const base = baseUrl();
    const q = token ? `?token=${encodeURIComponent(token)}` : "";
    return `${base}/activities/stream${q}`;
  },

  exportView: (page: string, body: { range?: string; format?: string }, token?: string) =>
    crmFetch(`/exports/views/${encodeURIComponent(page)}`, {
      method: "POST",
      body: JSON.stringify(body),
      token,
    }),

  tickets: listResource<unknown>("/tickets"),
  createTicket: createResource<unknown>("/tickets"),
  patchTicket: (id: string, body: unknown, token?: string) =>
    crmFetch(`/tickets/${id}`, { method: "PATCH", body: JSON.stringify(body), token }),

  campaigns: listResource<unknown>("/campaigns"),
  createCampaign: createResource<unknown>("/campaigns"),

  // Marketing OS
  marketingHub: (token?: string) => crmFetch("/marketing/hub", { token }),
  demandGen: (token?: string) => crmFetch("/marketing/demand-gen", { token }),
  segments: listResource<unknown>("/segments"),
  createSegment: createResource<unknown>("/segments"),
  journeys: listResource<unknown>("/journeys"),
  createJourney: createResource<unknown>("/journeys"),
  personas: listResource<unknown>("/personas"),
  createPersona: createResource<unknown>("/personas"),
  events: listResource<unknown>("/events"),
  createEvent: createResource<unknown>("/events"),
  contentAssets: listResource<unknown>("/content/assets"),
  createContentAsset: createResource<unknown>("/content/assets"),
  emailSends: listResource<unknown>("/email/sends"),
  createEmailSend: createResource<unknown>("/email/sends"),
  socialPosts: listResource<unknown>("/social/posts"),
  createSocialPost: createResource<unknown>("/social/posts"),
  seoKeywords: listResource<unknown>("/seo/keywords"),
  createSeoKeyword: createResource<unknown>("/seo/keywords"),
  budgetPlans: listResource<unknown>("/marketing/budget"),
  createBudgetPlan: createResource<unknown>("/marketing/budget"),
  mqls: listResource<unknown>("/mqls"),
  brandKit: (token?: string) => crmFetch("/brand-kit", { token }),

  // Engagement / loyalty
  loyaltyTiers: (token?: string) => crmFetch<{ data: unknown[] }>("/loyalty/tiers", { token }),
  loyaltyTierRules: (token?: string) => crmFetch("/loyalty/tier-rules", { token }),
  putLoyaltyTierRules: (body: unknown, token?: string) =>
    crmFetch("/loyalty/tier-rules", { method: "PUT", body: JSON.stringify(body), token }),
  loyaltyOutlets: listResource<unknown>("/loyalty/outlets"),
  createLoyaltyPromotion: createResource<unknown>("/loyalty/promotions"),

  // DMS bridge
  bridgeStatus: (token?: string) => crmFetch("/bridge/status", { token }),
  bridgeSync: (token?: string) => crmFetch("/bridge/sync", { method: "POST", token }),
  bridgeStreams: (token?: string) => crmFetch("/bridge/streams", { token }),
  bridgePendingImports: (token?: string) => crmFetch("/bridge/pending-imports", { token }),
  assignPendingImport: (id: string, owner: string, token?: string) =>
    crmFetch(`/bridge/pending-imports/${id}/assign`, {
      method: "POST",
      body: JSON.stringify({ owner }),
      token,
    }),
  bridgeMappings: (token?: string) => crmFetch("/bridge/mappings", { token }),
  bridgeSyncLog: (token?: string) => crmFetch("/bridge/sync-log", { token }),
  outlets: listResource<unknown>("/outlets"),
  outlet360: (id: string, token?: string) => crmFetch(`/outlets/${id}/360`, { token }),
  exportCustomers: listResource<unknown>("/export-customers"),
  createExportCustomer: createResource<unknown>("/export-customers"),

  // Intelligence
  insightsSummary: (token?: string) => crmFetch("/insights/summary", { token }),
  insightsSignals: (token?: string) => crmFetch("/insights/signals", { token }),
  seoAudit: (token?: string) => crmFetch("/seo/audit", { method: "POST", token }),
  aiSuggestions: (token?: string) => crmFetch("/ai/suggestions", { token }),
  aiCopilotChat: (message: string, token?: string) =>
    crmFetch("/ai/copilot/chat", { method: "POST", body: JSON.stringify({ message }), token }),

  audit: (token?: string) => crmFetch("/audit", { token }),
  adminMonitoringSummary: (token?: string) => crmFetch("/admin/monitoring/summary", { token }),
  adminAuditLogs: (token?: string) => crmFetch("/admin/audit-logs", { token }),
};

/** Map sidebar page ids (index.html) to primary data loaders for Next.js routes. */
export const CRM_PAGE_LOADERS = {
  overview: (token?: string) => crmApi.overview("week", token),
  pipeline: (token?: string) => crmApi.pipelineBoard(token),
  accounts: (token?: string) => crmApi.accounts(token),
  contacts: (token?: string) => crmApi.contacts(token),
  leads: (token?: string) => crmApi.leads(token),
  deals: (token?: string) => crmApi.deals(token),
  quotes: (token?: string) => crmApi.quotes(token),
  activities: (token?: string) => crmApi.activities(token),
  campaigns: (token?: string) => crmApi.campaigns(token),
  loyalty: (token?: string) => Promise.all([crmApi.loyaltyTiers(token), crmApi.loyaltyOutlets(token)]),
  tickets: (token?: string) => crmApi.tickets(token),
  bridge: (token?: string) =>
    Promise.all([
      crmApi.bridgeStatus(token),
      crmApi.bridgeStreams(token),
      crmApi.bridgePendingImports(token),
      crmApi.bridgeMappings(token),
      crmApi.bridgeSyncLog(token),
    ]),
  outlet360: (token?: string) => crmApi.outlets(token),
  export360: (token?: string) => crmApi.exportCustomers(token),
  insights: (token?: string) => crmApi.insightsSummary(token),
  ai: (token?: string) => crmApi.aiSuggestions(token),
  marketingHub: (token?: string) => crmApi.marketingHub(token),
  segments: (token?: string) => crmApi.segments(token),
  journeys: (token?: string) => crmApi.journeys(token),
  contentLib: (token?: string) => crmApi.contentAssets(token),
  brandKit: (token?: string) => crmApi.brandKit(token),
  mqls: (token?: string) => crmApi.mqls(token),
  events: (token?: string) => crmApi.events(token),
  demandGen: (token?: string) => crmApi.demandGen(token),
  emailStudio: (token?: string) => crmApi.emailSends(token),
  social: (token?: string) => crmApi.socialPosts(token),
  webSeo: (token?: string) => crmApi.seoKeywords(token),
  budget: (token?: string) => crmApi.budgetPlans(token),
  personas: (token?: string) => crmApi.personas(token),
} as const;
