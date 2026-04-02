# Zagforge — Dashboard (cloud.zagforge.com) [Phase 5]

The dashboard is a Next.js application living at `apps/cloud` inside the existing `zigzag-web` Turborepo monorepo (`~/Projects/zigzag-web`). It shares the `@workspace/ui` component library, `ThemeProvider`, and Tailwind v4 config with `apps/web` (zagforge.com) and `apps/docs` (docs.zagforge.com).

---

## Deployment

- **Domain:** `cloud.zagforge.com`
- **Platform:** Vercel (separate project, same monorepo — same pattern as `apps/web` and `apps/docs`)
- **Turbo pipeline:** add `apps/cloud` to `turbo.json` with the same `build`, `dev`, `lint`, `typecheck` tasks
- **Env vars:** `NEXT_PUBLIC_ZITADEL_ISSUER_URL` (e.g. `https://auth.zagforge.com`), `NEXT_PUBLIC_ZITADEL_CLIENT_ID`, `NEXT_PUBLIC_API_URL` (points to `api.zagforge.com`), `NEXT_PUBLIC_COOKIE_DOMAIN=.zagforge.com`

---

## Theme Sync

`NEXT_PUBLIC_COOKIE_DOMAIN=.zagforge.com` is already in the `turbo.json` env list. Drop `ThemeProvider` and `themeSyncScript` from `@workspace/ui` into `apps/cloud/app/layout.tsx` — identical pattern to `apps/web`. Dark/light preference syncs across all three subdomains with a shared cookie, zero extra work.

---

## Tech Stack

| Concern | Choice |
|---------|--------|
| Framework | Next.js 16 (App Router) |
| React | React 19 (React Compiler handles memoization) |
| Auth | Zitadel OIDC (Authorization Code + PKCE via `oidc-client-ts` or `next-auth` with Zitadel provider) |
| Data fetching | TanStack Query v5 |
| UI components | `@workspace/ui` (shadcn + Base UI + Radix + Tailwind v4) |
| Styling | Tailwind v4 (shared postcss config from `@workspace/ui`) |
| Animations | Motion (already in `apps/web`) |
| SSR | Used for initial repo list and snapshot data; interactive UI hydrates client-side |

**React 19 note:** No manual `useMemo`/`useCallback`. The React Compiler handles memoization automatically. Write straightforward component logic.

---

## Route Structure

```
apps/cloud/
  app/
    (auth)/                          ← no sidebar, centered card layout
      sign-in/page.tsx               ← redirects to Zitadel login (or custom form, Option B)
      sign-up/page.tsx               ← redirects to Zitadel register (or custom form, Option B)
      callback/page.tsx              ← OIDC callback handler (exchanges code for tokens)
      layout.tsx

    (dashboard)/                     ← authenticated, sidebar + workspace switcher
      layout.tsx
      page.tsx                       ← redirect → /repos

      repos/
        page.tsx                     ← repo list; shows onboarding-wizard if empty
        [[...repo]]/
          page.tsx                   ← Context URL panel + token table + Query Console

      account/
        page.tsx                     ← profile: username, email, phone, avatar
        sessions/page.tsx            ← active sessions list with revoke

      orgs/
        new/page.tsx                 ← create organization
        [orgSlug]/
          settings/page.tsx          ← org name, slug
          members/page.tsx           ← member list, invite, roles
          audit-log/page.tsx         ← org audit log

      settings/
        ai-keys/page.tsx             ← add/remove OpenAI, Anthropic, Google, xAI keys
        billing/page.tsx             ← "Pro features coming soon" stub

    not-found.tsx                    ← custom 404
```

**`[[...repo]]` catch-all:** GitHub repos are `owner/repo`. A single `[repo]` dynamic segment cannot capture a slash. The catch-all handles `cloud.zagforge.com/repos/zagforge/zigzag` naturally and is future-proof for monorepo sub-paths.

**URL convention:** `cloud.zagforge.com/repos/{owner}/{repo-name}` — the first segment is either the username (personal workspace) or the org slug (org workspace). The frontend resolves this from the active workspace context.

---

## Feature Directory

```
features/
  auth/components/
    auth-callback.tsx              ← handles OIDC redirect callback
    username-prompt.tsx            ← shown on first SSO login when username not set

  repos/components/
    repo-card.tsx
    snapshot-badge.tsx
    empty-state.tsx

  onboarding/components/
    onboarding-wizard.tsx            ← shown when user has zero repos

  context/components/
    context-url-panel.tsx            ← hero element, above the fold
    token-table.tsx
    copy-button.tsx                  ← "Copy for Cursor" + "Copy for Claude"

  chat/components/
    query-console.tsx
    message-bubble.tsx
    sse-stream-hook.ts

  account/components/
    profile-form.tsx               ← edit username, email, phone
    session-list.tsx               ← active sessions with revoke buttons
    delete-account-dialog.tsx

  orgs/components/
    org-create-form.tsx
    member-table.tsx
    invite-dialog.tsx
    role-selector.tsx
    audit-log-table.tsx

  settings/components/
    ai-key-form.tsx
    key-hint-row.tsx
    billing-stub.tsx
```

---

## Auth: Zitadel OIDC

All auth flows use standard OIDC with Zitadel as the provider. Two approaches depending on the desired UX:

**Option A: Zitadel hosted login (recommended for launch)**
- Redirect to `auth.zagforge.com` for login/signup — Zitadel renders the login page (custom branded via Zitadel's branding settings)
- After auth, redirect back to `cloud.zagforge.com` with authorization code
- Next.js exchanges code for tokens via PKCE
- Simplest to implement, handles all edge cases (MFA, account linking, password reset)

**Option B: Custom login pages (future)**
- Build fully custom sign-in/sign-up forms in Next.js
- Use Zitadel's session API to authenticate directly
- Full UI control, more implementation effort

**Auth library:** `oidc-client-ts` (lightweight, framework-agnostic) or `next-auth` v5 with Zitadel OIDC provider.

**Middleware:** `middleware.ts` checks for a valid session token on all `(dashboard)` routes. If missing or expired, redirects to the Zitadel login flow. Auth routes in `(auth)` are public.

The Go API validates `Authorization: Bearer <Zitadel JWT>` on every authenticated request via JWKS-based local verification.

---

## Onboarding Wizard

When `repos/page.tsx` receives an empty repo list, it renders `onboarding-wizard.tsx` instead of an empty table. The wizard shows:

```bash
# 1. Install Zigzag
brew install zagforge/tap/zigzag

# 2. Set your API key
export ZAGFORGE_API_KEY=zf_pk_...

# 3. Upload your first snapshot
cd your-project && zigzag --upload
```

The API key is pre-filled from the user's session (fetched client-side). This turns the empty state into a Quick Start — not a dead end.

---

## Context URL Panel (Hero Element)

On `[[...repo]]/page.tsx`, the context URL panel is **above the fold**:

```
┌─────────────────────────────────────────────────────┐
│  Context URL                                         │
│  ──────────────────────────────────────────────────  │
│  https://api.zagforge.com/v1/context/zf_ctx_...      │
│                                                      │
│  [Copy for Cursor]    [Copy for Claude]    [Revoke]  │
│                                                      │
│  Last fetched: 2 hours ago  ·  Snapshot: main@3fa9  │
└─────────────────────────────────────────────────────┘
```

Below it: the token management table (all tokens for this repo, create/revoke).

Below that: the Query Console (same page, no navigation required).

---

## Query Console (SSE)

The Query Console streams responses from the Go backend via Server-Sent Events.

```ts
// lib/sse.ts
export function useSSEQuery(url: string, body: object) {
  const [messages, setMessages] = useState<string[]>([])

  async function submit() {
    const res = await fetch(url, { method: 'POST', body: JSON.stringify(body),
      headers: { 'Content-Type': 'application/json', 'Accept': 'text/event-stream' } })
    const reader = res.body!.getReader()
    const decoder = new TextDecoder()
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      setMessages(prev => [...prev, decoder.decode(value)])
    }
  }

  return { messages, submit }
}
```

TanStack Query is used for the initial data fetches (repo list, snapshot list, token list). The SSE query stream is handled via the custom hook above, not TanStack Query (which does not model streaming).

---

## Workspace Switcher & Multi-Tenancy

The sidebar includes a workspace switcher that shows:

1. **Personal** (default) — the user's personal workspace, always present
2. **Organizations** — any orgs the user has created or been invited to (zero or more)

Switching workspace changes the active scope for all API requests (personal `user_id` vs. org `org_id`). The active scope is persisted in a cookie or local storage so it survives page reloads.

- **Org creation:** Custom form → `POST /api/v1/orgs` → creates in Zitadel + local DB
- **Invite flow:** Org admin invites via email → Go API sends invite email → invitee accepts → membership created
- **RBAC:** Roles (owner/admin/member) stored in `memberships` table, enforced by Go API middleware. Fine-grained per-repo RBAC is a future phase.
- **Billing tab:** renders `billing-stub.tsx` — "Pro features coming soon. Contact us for early access."

---

## Page-Level SSR Strategy

| Route | Rendering |
|-------|-----------|
| `(auth)/*` | Client-side (no sensitive data, fast auth redirects) |
| `repos/page.tsx` | SSR — initial repo list hydrated on server for fast first paint |
| `[[...repo]]/page.tsx` | SSR for snapshot metadata; Context URL panel and Query Console hydrate client-side |
| `settings/*` | Client-side (small payloads, always behind auth) |
| `not-found.tsx` | Static |

---

## Quick Reference: Architecture Docs

| Doc | Phase | Topic |
|-----|-------|-------|
| [15-context-proxy.md](15-context-proxy.md) | 5 | Context URL, assembly pipeline, Query Console, token lifecycle |
| [17-cli-upload.md](17-cli-upload.md) | 5 | `zigzag --upload` CLI flag, upload endpoint, CLI token auth |
| [02-data-model.md](../phase1/02-data-model.md) | 1+5 | All tables including Phase 5 additions |
| [04-api-endpoints.md](../phase2/04-api-endpoints.md) | 2+5 | All API endpoints including Phase 5 additions |
| [05-authentication.md](../phase2/05-authentication.md) | 2+5 | Auth mechanisms including CLI token and context token auth |
