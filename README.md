# Replacement Copilot - step-by-step guide

A study project for one architecture pattern: a **tool-using AI agent behind a backend service**, where *the model reasons, the backend decides, and a person approves*.

The example domain is car replacement: when a customer's car is in repair, an **AI agent** finds a rental car that their insurer's or leasing company's contract covers. It runs on **Claude Managed Agents**, every tool is a **Go** function with hard guardrails, a person approves each booking, and a **React + TypeScript** UI shows what the agent did and why. The same design fits claims handling, order support, ticket triage, or any process where an AI does the work and a person stays accountable.

All data (partners, customers, fleet, contracts) is made up.

> **New to Go or to AI agents?** Read the [architecture study guide](docs/ARCHITECTURE.md). It explains every concept from zero, follows one case through the code step by step, and has a Go-for-TypeScript-developers section. This README covers how to run it.

---

## 1. What the pattern covers, and where in the code

| Concept | Where it is in the code |
|---|---|
| Go microservice, cloud-native, event-driven | `backend/` Go service: domain store, HTTP + SSE API, a `ReplacementReserved` domain event on an in-process bus (stands in for Kafka) consumed by a second "service" |
| AI agent orchestration (multi-step) | `internal/agentrun`: drives a Managed Agents session (stream events, run custom tools in parallel, send results, reconnect + dedupe) |
| RAG | `internal/rag`: BM25 over partner contracts + ops playbook, chunks have stable IDs, and the agent must cite them |
| Context management | The agent works only through small typed tools that return compact JSON. Managed Agents stores the session state on the server, so the browser and Go service don't hold the conversation. |
| Evaluation | `cmd/eval`: runs all 3 cases end to end and checks fixed rules on the trace and the booking |
| Guardrails | `domain.CheckProposal`: Go decides whether a proposal is allowed (no downgrade, policy cap, max days, automatic, availability). The model can only propose. |
| Human in the loop | `agentrun/approver.go`: the booking tool call stays open until a person approves or rejects it in the UI |
| Transparent UI for business users | `frontend/` (small functional components in `src/components`, state in `src/hooks`): conversation, approval card, and a live trace of tool calls, retrieved policy text, guardrail results and timings |
| Spec-first development with AI coding agents | This README is the spec. The code was built from it with **Claude Code** |

### The three demo cases

| Case | Situation | What a good agent does |
|---|---|---|
| **RC-1001** Lisbon, NordInsure | Nearest branch has no free COMPACT on the start date | Finds the COMPACT at the *other* Lisbon branch. It does not downgrade to the Clio or upgrade to the 308 |
| **RC-1002** Munich, FleetLease | Family, automatic only, 14 days | Skips the manual Passat and proposes an automatic (the X1 one category up, or the 7-seat Touran) |
| **RC-1003** Lisbon, NordInsure | Customer wants an SUV, repair takes 12 days, policy allows COMPACT and 10 days | Books a covered car for 10 days, flags an extension request, and offers the SUV only as a co-pay option. Doesn't assume consent |

---

## 2. Architecture

```
 React + TS (Vite :5173)                Go service (:8080)                         Anthropic (Managed Agents)
 ┌──────────────────────┐   REST      ┌──────────────────────────────┐   SDK      ┌──────────────────────────┐
 │ Cases / Conversation │ ─────────▶  │ cmd/server  HTTP + SSE hub   │ ─────────▶ │ Agent "Replacement       │
 │ Approval card        │             │ internal/agentrun            │  sessions  │ Copilot" (versioned:     │
 │ Agent trace timeline │ ◀───SSE──── │   stream ⇄ dispatch tools    │ ◀──SSE──── │ model, prompt, 4 custom  │
 └──────────────────────┘             │ internal/tools  (4 tools)    │            │ tools, no sandbox tools) │
                                      │ internal/domain guardrails   │            │ Session = 1 case convo   │
                                      │ internal/rag    BM25 + cites │            └──────────────────────────┘
                                      │ internal/bus    domain events│──▶ partner-notification-svc (log)
                                      └──────────────────────────────┘
```

**One tool call, step by step:**
1. The agent emits `agent.custom_tool_use` (for example `search_vehicles`) on the session's SSE stream.
2. Go runs it in a goroutine, so the stream keeps being read and parallel calls work.
3. For `propose_replacement`, Go runs the guardrails first. A violation goes straight back to the agent as an error result, so it fixes the proposal and retries.
4. If the proposal passes, the tool call **stays open** while the UI shows an approval card. The session idles with `requires_action` until the person decides.
5. Go sends `user.custom_tool_result`. If the booking is approved, it saves the reservation and publishes `ReplacementReserved`.

---

## 3. Prerequisites (Windows, ~10 min)

1. **Go 1.25+**:
   ```powershell
   winget install GoLang.Go
   ```
   Open a new terminal, then check with `go version`.
2. **Node 20+**: you already have Node 24.
3. **Credentials.** If `ant auth status` shows an `(active)` credential, you're done: the Go SDK uses your `ant auth login` sign-in automatically. Don't set `ANTHROPIC_API_KEY` in that case, because a set key (even a wrong one) takes priority over the sign-in. Only if you have no sign-in, create a key at platform.claude.com → Settings → API keys and set it in the terminal with `$env:ANTHROPIC_API_KEY = "<your key>"`. Never commit the key.

> **Cost and beta:** Managed Agents is in beta. You pay for tokens plus session runtime, and session history is kept until you delete it. A full demo run of the 3 cases is a few agent turns each, so it's cheap, but don't leave loops running.

> **If you already have agents in the Console:** this project needs an agent whose tools are the 4 custom Go tools, so `cmd/setup` creates its own agent called "Replacement Copilot". Your other agents stay untouched. Every session this project starts also shows up in the Console session viewer, and the UI header links to it. That's the best place to see the full server-side transcript.

---

## 4. Step 1: create the agent once (control plane)

```powershell
cd backend
go mod tidy
go run ./cmd/setup
```

Expected output:
```
created environment env_...
created agent agent_... (version 1)
saved agent.json
```

`agent.json` stores the IDs. **Agents are persistent and versioned**: you create one once and reuse its ID for every session. If you change `SystemPrompt` or a tool in `internal/tools/tools.go`, run `go run ./cmd/setup` again. That **updates** the agent to version 2, 3, and so on, and sessions already running keep their version. That's what makes safe rollouts and A/B tests of agent versions possible.

## 5. Step 2: run the Go service

```powershell
go run ./cmd/server
```
```
agent agent_... v1, environment env_...
listening on http://localhost:8080
```

Quick check in another terminal:
```powershell
curl.exe http://localhost:8080/api/cases
```

## 6. Step 3: run the UI

```powershell
cd ..\frontend
npm install
npm run dev
```
Open http://localhost:5173.

## 7. Step 4: walk through the three cases (~5 min)

1. **RC-1001, the happy path.** Click *Run copilot on RC-1001*. Narrate the right-hand trace as it fills: `get_case` → `search_policies` (open a chunk to show the retrieved contract text) → `search_vehicles` → `propose_replacement` → guardrails **passed** → an approval card appears. Click **Approve**. The agent's final answer has citation chips, and *Confirmed reservations* updates. In the Go terminal, the `[bus] ReplacementReserved consumed by partner-notification-svc` line is the event-driven part.
2. **RC-1003, the guardrails.** Run it. If the agent tries the SUV or 12 days, the trace shows **Guardrails: blocked** with the reasons, and the agent fixes it on its own. The point to notice: the model proposes and Go decides. Business rules are code, tested and audited, not prompt text.
   Then type in the composer: *"The customer agreed to pay the difference for the SUV, 10 days."* The agent proposes `LIS-CTR-03` with `customer_copay=true`. **Reject** it with a note like "co-pay needs written consent" to show that a person's feedback goes back into the agent's loop.
3. **Open the session in the Console** (header link) to show the full server-side transcript: the same events your Go code consumed.
4. **Run the eval** (next step) or show its output from an earlier run.

> Tip: if you show this to others, run every case once beforehand. Save the eval output and a short screen recording in case the network or API fails during the demo.

## 8. Step 5: evaluation

```powershell
cd backend
go run ./cmd/eval
```
Sample output format (the actual results come from your own run):
```
== RC-1003  (5 tool calls, 1 guardrail rejections, 41s)
   tools: get_case -> search_policies -> search_vehicles -> propose_replacement -> propose_replacement
   [PASS] loads the case first
   [PASS] reads policy before proposing
   [PASS] ends with a confirmed reservation
   [PASS] cites policy chunks in final answer
   [PASS] caps duration at 10 days
   [PASS] does not assume co-pay consent
   [PASS] flags the extension request
```
The eval uses an auto-approver and a fresh fleet for each case. All checks are deterministic, with no LLM judge, so a failure points to a specific behaviour. In production you would add: many cases per scenario, repeated runs to measure pass rate (agents are non-deterministic), an LLM-judge rubric for answer quality, and running the eval in CI on every change to the prompt or tools before you bump the agent version.

## 9. Step 6: iterate like a spec-first team

1. Change the spec, for example add "prefer delivery to garage when the policy covers it" to the playbook and the prompt.
2. Re-run `go run ./cmd/setup`, which creates a new agent version.
3. Run `go run ./cmd/eval` and compare with the last results.
4. Only then ship it. This is the loop: **spec → Claude Code implements → eval gates the agent version**.

---

## 10. Design decisions

The [study guide](docs/ARCHITECTURE.md) explains each one in depth.

- **Custom tools only, no sandbox tools.** The agent has no bash and no web. It can only call 4 typed Go functions, which keeps data access and side effects in the backend (least privilege). Managed Agents still runs the agent loop, keeps session state and gives you the session viewer.
- **Deterministic guardrails apart from the LLM.** The policy exists twice: as prose for RAG (what the agent reasons with) and as a struct for `CheckProposal` (what is enforced). The model can't bypass the guardrails by writing something clever.
- **Human in the loop as an open tool call.** Approval is not a separate flow. The `propose_replacement` result stays pending until a person decides, so the agent sees the decision and the note as a normal tool result.
- **RAG with citations.** The agent must cite chunk IDs, and the UI shows the exact retrieved text. A business user can check every "why" against the source. BM25 is fine for 3 docs. At scale: embeddings + pgvector, hybrid search, re-ranking, and an eval set for retrieval quality.
- **Reliable stream handling.** The SSE stream has no replay. On every (re)connect the Go code opens the stream first, then lists past events and dedupes by event ID, and it re-runs only tool calls that never got a result. Without this, a dropped connection during a pending tool call would leave the session stuck.
- **Event-driven.** Confirming a booking publishes `ReplacementReserved`, which other services (partner notification, billing) consume. In production: Kafka, the outbox pattern for exactly-once publishing from the DB transaction, and idempotent consumers.
- **Versioned agents.** The prompt and tools are a versioned config. Roll out by creating a new version, gate it with evals, and roll back by pinning sessions to the previous version.
- **Transparent UI.** Business users see the *what* (conversation, approval card) and engineers or ops see the *how* (the trace timeline). Both come from one normalized `Trace` event schema produced in Go.

## 11. What I'd add for production

- Session budgets (a hard spend cap per session) and per-tenant rate limits
- OpenTelemetry traces across the Go service and the Managed Agents sessions
- AuthN/Z on the API, and an audit log of every approval
- Real services behind the tools (fleet availability, partner contracts, the case system), with timeouts, retries and idempotency keys on `propose_replacement`
- Webhooks instead of long-lived streams for very long sessions
- A larger eval set from real anonymised cases, tracking pass rate over time

## 12. Troubleshooting

| Symptom | Fix |
|---|---|
| `read agent.json ... run go run ./cmd/setup first` | Run step 1 from the `backend` folder |
| `401 authentication_error` | No valid credential: run `ant auth login`, or check that `ANTHROPIC_API_KEY` in *this* terminal is a real key and not a placeholder |
| Go compile error on an SDK type name | The SDK is beta and its names change. Check the installed version with `go list -m github.com/anthropics/anthropic-sdk-go/v2` (the code was written against **v2.15.0**). Pin that version in `go.mod` or fix the name the compiler points to |
| UI shows nothing after *Run copilot* | Check that the Go server is running on :8080 (Vite proxies `/api`) and look at its terminal for errors |
| Approval card stays forever | The click goes to `POST /api/approvals/{id}`. A server restart drops pending approvals, so start a new session |

## Project layout

```
docs/
  ARCHITECTURE.md  study guide: concepts, step-by-step flow, Go for TS developers
backend/
  cmd/setup      create/update environment + agent  -> agent.json
  cmd/server     HTTP + SSE API for the UI
  cmd/eval       end-to-end scenario checks
  internal/agentrun  session orchestration, approvals, Trace schema
  internal/tools     tool schemas, system prompt, execution
  internal/domain    cases, policies, fleet, guardrails (mock data)
  internal/rag       BM25 retrieval over policies/*.md
  internal/bus       in-process domain events
frontend/src/
  App.tsx        composition only: wires hooks to components
  types.ts       mirrors the Go Trace / domain structs
  hooks/         useSession (start + status), useTraces (SSE + dedupe), useFetch
  lib/           api (post), format (tool args)
  components/    small functional components, one per file:
    Header, StatusPill, Panel
    CaseList, CaseCard, RunButton, ReservationList          (left: cases)
    Conversation, MessageItem, ApprovalCard,                (middle: business view)
      ApprovalDetails, ApprovalOutcome, Composer, RichText
    Timeline, TimelineItem, GuardrailVerdict,               (right: transparency view)
      ToolOutput, PolicyHits, VehicleTable, RawOutput
```
