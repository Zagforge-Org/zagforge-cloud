# Zagforge — Dashboard (cloud.zagforge.com) [Phase 5]

The dashboard is a Next.js application living at `apps/cloud` inside the existing `zigzag-web` Turborepo monorepo (`~/Projects/zigzag-web`). It shares the `@workspace/ui` component library, `ThemeProvider`, and Tailwind v4 config with `apps/web` (zagforge.com) and `apps/docs` (docs.zagforge.com).

---

## Deployment

- **Domain:** `cloud.zagforge.com`
- **Platform:** Vercel (separate project, same monorepo — same pattern as `apps/web` and `apps/docs`)
- **Turbo pipeline:** add `apps/cloud` to `turbo.json` with the same `build`, `dev`, `lint`, `typecheck` tasks
- **Env vars:** `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `NEXT_PUBLIC_API_URL` (points to `api.zagforge.com`), `NEXT_PUBLIC_COOKIE_DOMAIN=.zagforge.com`

---

## Theme Sync

`NEXT_PUBLIC_COOKIE_DOMAIN=.zagforge.com` is already in the `turbo.json` env list. Drop `ThemeProvider` and `themeSyncScript` from `@workspace/ui` into `apps/cloud/app/layout.tsx` — identical pattern to `apps/web`. Dark/light preference syncs across all three subdomains with a shared cookie, zero extra work.

---

## Tech Stack

| Concern | Choice |
|---------|--------|
| Framework | Next.js 16 (App Router) |
| React | React 19 (React Compiler handles memoization) |
| Auth | Clerk headless (`useSignIn`, `useSignUp`, `useOrganization` hooks) |
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
      sign-in/page.tsx
      sign-up/page.tsx
      forgot-password/page.tsx
      verify/page.tsx                ← email verification step after sign-up
      layout.tsx

    (dashboard)/                     ← authenticated, sidebar + org switcher
      layout.tsx
      page.tsx                       ← redirect → /repos

      repos/
        page.tsx                     ← repo list; shows onboarding-wizard if empty
        [[...repo]]/
          page.tsx                   ← Context URL panel + token table + Query Console

      settings/
        ai-keys/page.tsx             ← add/remove OpenAI, Anthropic, Google keys
        team/page.tsx                ← Clerk OrganizationSwitcher + member list
        billing/page.tsx             ← "Pro features coming soon" stub

    not-found.tsx                    ← custom 404
```

**`[[...repo]]` catch-all:** GitHub repos are `owner/repo`. A single `[repo]` dynamic segment cannot capture a slash. The catch-all handles `cloud.zagforge.com/repos/zagforge/zigzag` naturally and is future-proof for monorepo sub-paths.

**URL convention:** `cloud.zagforge.com/repos/{clerk-org-slug}/{repo-name}` — the first segment is the Clerk org slug (not the GitHub owner name, which may differ). The frontend resolves the Clerk org slug from the active session.

---

## Feature Directory

```
features/
  auth/components/
    sign-in-form.tsx
    sign-up-form.tsx
    forgot-password-form.tsx
    verify-form.tsx

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

  settings/components/
    ai-key-form.tsx
    key-hint-row.tsx
    billing-stub.tsx
```

---

## Auth: Clerk Headless

All auth pages use Clerk's React hooks directly — **no Clerk-branded UI components**. Forms are standard HTML/React styled with Tailwind and `@workspace/ui` primitives.

```tsx
// features/auth/components/sign-in-form.tsx
import { useSignIn } from '@clerk/nextjs'

export function SignInForm() {
  const { signIn, isLoaded } = useSignIn()

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const { identifier, password } = Object.fromEntries(new FormData(e.currentTarget))
    await signIn.create({ identifier: String(identifier), password: String(password) })
  }

  // ...standard form with Input, Button from @workspace/ui
}
```

The Go API is unchanged — it still validates `Authorization: Bearer <Clerk JWT>` on every authenticated request.

**Clerk Middleware:** `clerkMiddleware()` in `apps/cloud/middleware.ts` protects all `(dashboard)` routes. Auth routes in `(auth)` are public.

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

## Multi-Tenancy: Orgs Now, Billing Later

Clerk Organizations are enabled in Phase 5. The data model already has `org_id` on all relevant tables.

- **Org switcher:** `<OrganizationSwitcher />` (Clerk component) in the sidebar layout
- **Invite flow:** Clerk's built-in invite UI — emails are sent by Clerk, no custom invite endpoint needed
- **Billing tab:** renders `billing-stub.tsx` — "Pro features coming soon. Contact us for early access." Collects lead emails passively.
- **RBAC:** Clerk org roles (admin/member) are respected in Phase 5; fine-grained per-repo RBAC is a future phase.

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
