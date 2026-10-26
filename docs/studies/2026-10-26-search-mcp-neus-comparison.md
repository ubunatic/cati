---
kind: topic
title: "Session Study: Codebase Memory MCP versus neus ask"
date: 2026-10-26
status: session-complete
baseline: b1f50be
---

# Session Study: Codebase Memory MCP versus neus ask

## Context

The owner requested a session-only comparison: retain codebase-memory-mcp for structural exploration, and pair every MCP call made after that request with `neus ask` addressing the same question. This is an experiment for this session, not a repository policy or a default for future sessions. Append subsequent comparisons here while the session continues.

The first case comes from [lint epic 120](../../issues/120-epic-reconcile-unused-go-declarations-across-cli-and-render-packages.md): lint labels `parseYamlView` unused, but previous graph exploration found a test caller. Four MCP calls were paired with four Neus calls, without retries or query tuning. Earlier session MCP calls predate the request and are not paired or scored here.

The [raw results](2026-10-26-search-mcp-neus-comparison.json) preserve questions, exact MCP tool arguments, returned evidence, Neus answers and sources, actual backend/model, and timing. The MCP arguments for each call are also recorded below; the JSON records the resulting output rather than a full tool invocation envelope. Neus ran from the Cati root with `HTO=0 neus ask --tests --json '<question>'`, default retrieval settings, and no explicit backend override. `--tests` was intentional because the question includes test callers.

## Outcome

### Round 1 — Locate the symbol

**Question:** Where is parseYamlView defined in this cati repository? Identify its exact file and function symbol; include tests if relevant.

- MCP: `search_graph(project="home-uwe-projects-cati", name_pattern="^parseYamlView$", limit=20)`. Returned exactly one match, `home-uwe-projects-cati.cmd.parseYamlView`, at `cmd/browser_config.go:411–423`, with one incoming edge. No pagination remained. Elapsed: **123ms**.
- Neus: correctly identified `func parseYamlView(name string) (*YamlView, error)` in that file, and additionally identified `TestBrowser_ParseYaml` at `cmd/browser_test.go:143`. Both claims had relevant numbered citations. Total reported wall time: **7,439ms**, including retrieval **1,831ms**.
- Judgment: both answered the locator question correctly. Neus supplied a useful test detail without a separate caller question, although three of its five sources were unrelated configuration summaries.

### Round 2 — Find callers, including tests

**Question:** Who calls parseYamlView in cati, including tests? Distinguish test callers from production callers and state the limits of any no-production-callers claim.

- MCP: `trace_path(project="home-uwe-projects-cati", function_name="home-uwe-projects-cati.cmd.parseYamlView", direction="inbound", include_tests=true, depth=2, max_output_tokens=2000)`. Returned one caller, `TestBrowser_ParseYaml`, marked `test=true`, with an exact total of one graph caller. Elapsed: **133ms**.
- Neus: could not identify any callers because its retrieved excerpts did not mention the symbol or its call graph. It explicitly refused to conclude there were no production callers. Its four sources included performance guidance, view YAML and unrelated test chunks. Wall: **5,532ms**, retrieval **1,582ms**.
- Judgment: MCP supplied the actionable caller evidence; Neus did not answer, but correctly limited its claim. Notably, Neus had found the relevant test in round 1 and lost it when the caller question changed. This is a retrieval robustness observation, not proof of a particular implementation bug.

### Round 3 — Read behavior and error handling

**Question:** What does parseYamlView do in cmd/browser_config.go, including error handling and delegation?

- MCP: `get_code_snippet(project="home-uwe-projects-cati", qualified_name="home-uwe-projects-cati.cmd.parseYamlView", max_output_tokens=2000)`. Returned the complete 13-line body: call `spec.LoadYamlView(name)`, propagate errors as `nil, err`, otherwise copy Type, Name, Title, Content and Controls into `*YamlView`. Elapsed: **125ms**.
- Neus: retrieved the correct symbol at line 411 but said its supplied excerpt cut off before the body, so behavior, error handling and delegation could not be determined. Wall: **5,092ms**, retrieval **779ms**.
- Judgment: MCP exposed sufficient source for the answer; Neus abstained despite identifying the right symbol. The body exists and is short. The saved answer does not reveal the actual prompt excerpt, so truncation, budgeting, stale indexed content and retrieval selection cannot yet be separated as causes. The MCP's speculative documentation suggestions were unnecessary to resolve the question and were not used as proof.

### Round 4 — Check index coverage and freshness

**Question:** Does the current codebase-memory index have parsing or freshness gaps for cmd/browser_config.go and cmd/browser_test.go? Separate live index metadata from inferences based on source retrieval.

- MCP: `check_index_coverage(project="home-uwe-projects-cati", paths=["cmd/browser_config.go", "cmd/browser_test.go"])`. Both paths returned `no_recorded_issue`, `metadata_match`, and the best-effort caveat. The reported generation was `2026-10-26T16:53:45Z`, full mode, generation matching, with no pending result pages. Elapsed: **127ms**.
- Neus: retrieved symbols from both files but explicitly stated that this did not establish live coverage or freshness. Wall: **4,950ms**, retrieval **896ms**. Two sources were unrelated generated `dist/config.yaml` chunks.
- Judgment: MCP provided its own live diagnostic; Neus properly recognized that its retrieved sources could not substitute for it. This is a capability difference, not a comparable failed code-answer task, and is excluded from the substantive answer tally.

### Source adjudication

Direct reads of `cmd/browser_config.go:411–423` and `cmd/browser_test.go:143–157` confirmed the function body, test name, and test call at line 144. `rg -n '\bparseYamlView\b' --glob '*.go' .` found exactly the declaration and that call. This supports the bounded conclusion that the checked Go sources contain one explicit test call and no other literal references. It does not establish an exhaustive absence of runtime/reflection/dynamic references.

Coverage for both evidence files had no recorded issue. That is a best-effort signal, not proof that the graph captures every possible relationship. MCP output sometimes reports different edge summaries across tools; this study relies on the explicit caller trace and checked source, not raw degree counts as a universal reference count.

### Timing and answer comparison

| Round | MCP elapsed | Neus reported wall | Neus retrieval | Useful answer |
|---|---:|---:|---:|---|
| Locate | 123ms | 7,439ms | 1,831ms | Both |
| Callers | 133ms | 5,532ms | 1,582ms | MCP; Neus abstained |
| Behavior | 125ms | 5,092ms | 779ms | MCP; Neus abstained |
| Coverage diagnostic | 127ms | 4,950ms | 896ms | MCP live metadata; Neus correctly limited scope |

For three substantive code questions, MCP supplied sufficient correct evidence in **3/3**; Neus supplied a full answer in **1/3**, with two explicit abstentions. No incorrect substantive answer was observed. Four related questions about one function are not a general benchmark, nor evidence that Neus always loses. MCP returns structured evidence and source; Neus returns synthesized answers, so timing compares different end products. Host-side MCP timing includes the awaited tool call; Neus wall time is its own telemetry, not the shell wrapper's entire lifetime.

All four Neus calls routed from the configured local path to **Codex `luna:low`** because estimated local chat time (~80–81 seconds) exceeded the default one-minute route threshold. Consequently, these are not measurements of the local Qwen answer quality or latency. Neus generation and TTFT telemetry were zero on that fallback path despite positive wall time: do not interpret those zeros as free or instantaneous generation. Cold/warm caches and ordering were not controlled.

Neus can consume Codebase Memory symbol evidence; this compares two access paths with partly shared infrastructure, not two independent ground-truth systems. No claims about model price, broad recall, reproducibility across hardware, or index-completeness guarantees follow from this sample.

## What Went Wrong

- Neus's caller query missed a test it had found in the locator query. Its behavior query found the function but did not expose its body. Both were caught by comparing the answers with MCP source/trace and direct file reads; no retrieval implementation fix was attempted.
- The local/backend routing expectation was wrong if read as a local-model test. Actual JSON reported `backend: codex`, `model: luna:low`; those facts govern the study, not the CLI's nominal default.
- The host initially added pairing instructions to `AGENTS.md`. The owner corrected the scope: session-only. The entire addition was removed immediately, and `git diff -- AGENTS.md` was empty. No persistent agent rule is part of this change.
- None of the Neus invocations exited unsuccessfully. An honest insufficient-evidence answer is a task-level miss, not a process crash or fabricated claim.

## Learnings

For this case, retain exact symbol search, an explicit test-inclusive caller trace, source snippets and coverage checks as separate evidence steps. A useful locator answer does not establish robust caller or function-body retrieval. Preserve abstentions rather than converting them into no-caller claims. Record the actual answering backend before comparing speed or model behavior.

For subsequent session queries, pair every MCP call with Neus on the same underlying question, preserve exact wording and options, and append results here. Treat any tuned or retried query as another recorded trial. This session protocol is documented as study methodology only; it does not amend repository instructions.

## Follow-Up Tickets

No new ticket allocated. The existing Neus evaluation task **046 — Evaluation and strategic integration plan for codebase-memory-mcp** already covers this comparison area; the current observations are research inputs, not a diagnosed root cause or a new implementation commitment. Existing Neus tickets 049 and 053 document related snippet/retrieval limitations but are Closed; this study does not silently reopen them or claim their fixes regressed.

The next research step within this session is to append naturally occurring paired questions, including cases outside this one test helper. Root-cause debugging of Neus, forced-local model runs, prompt capture, systematic retrieval tuning and a large benchmark are non-goals of this initial study. No workers were allocated; the host performed the follow-up review. Cati lint remediation remains in #120, separate from this comparison.

## Harnez issue 953 — additional session sample

The owner extended the pairing request while fixing Harnez issue 953 (interactive
Codex hosts inheriting leaf-worker roles). MCP calls before that message remain
outside the paired tally. One MCP call followed the message, paired as below.

**Question:** Does the codebase-memory index have parsing or freshness gaps for
`cmd/harnez/agent_caller.go`, `cmd/harnez/agent_caller_test.go`,
`internal/subagent/codex_host.go` and `internal/procs/clean.go`? Distinguish live
index metadata from source retrieval.

- MCP: `check_index_coverage(project="home-uwe-projects-harnez", paths=["cmd/harnez/agent_caller.go", "cmd/harnez/agent_caller_test.go", "internal/subagent/codex_host.go", "internal/procs/clean.go"])`.
  All four paths returned `no_recorded_issue` and `metadata_match`; generation
  `2026-10-26T17:15:24Z`, full mode, generation matching, four paths returned and
  no remaining pages. The best-effort caveat still applies. This included newly
  created, uncommitted files. MCP timing was not separately measured.
- Neus: `HTO=0 neus ask --tests --json '<question above>'`, run in the Harnez
  root. It correctly abstained from claiming live parsing/freshness information.
  Its only code source was the unrelated `cmd/harnez/agent_send.go:3`; other
  sources were `config.yaml:634`, `website/page.yaml:5`, and
  `internal/usage/livefetchcache.go:58`. Backend **codex**, model **luna:low**;
  wall **6,137.671ms**, retrieval **1,527.035ms**. Local routing was skipped
  because the estimated chat time was 81.3 seconds. Zero generation/TTFT
  telemetry on this fallback path remains unsuitable as a latency measurement.
- Judgment: MCP supplied its own live diagnostic; Neus accurately stated the
  capability limit. Excluded from the substantive code-answer tally, as in round
  4. No retry, source tuning or forced-backend run was performed.

An additional **unpaired exploratory Neus query**, excluded from the tally, asked:
"In /home/uwe/projects/harnez, how do guardLeafRole, startRole, resumeRole and
currentHostSession use HARNEZ_AGENT_ROLE and HARNEZ_SESSION_ID, and where is the
worker environment exported?" It ran as `neus ask '<question>'`. Neus described
`currentHostSession`'s interactive-host precedence, but explicitly could not
establish the other functions or environment export from its excerpts. Sources:
`spec/agent.yaml:203`, `config.yaml:721`, `cmd/harnez/agent_args.go:16`.
It routed to Codex luna:low after an 81.2-second local estimate; JSON timing was
not requested. The source evidence for those functions had already been read by
MCP and direct file reads before the owner's extended pairing instruction.

These observations introduce no new diagnosed Neus defect or follow-up ticket;
Neus evaluation 046 remains the research owner. The Harnez fix used a separate
read-only reviewer (`codex:luna:high`, session `issue953-review`), which reported
no findings and confirmed its leaf-role restriction live. That review is not a
retrieval comparison trial.

### Retrospective backfill of the seven earlier Harnez MCP calls

After the fix and review, the owner requested Neus answers for **all earlier
unpaired MCP calls**. All seven were run once with `HTO=0 neus ask --tests --json`
from the Harnez root. Together with the contemporaneous coverage pair above,
this accounts for **all eight MCP calls in the Harnez issue-953 session**.
Tool-discovery metadata listings were not MCP invocations. The extra exploratory
Neus query above remains unpaired and outside the comparison tally.

[Backfill raw results](2026-10-26-search-mcp-neus-harnez953-backfill.json) preserve
exact original MCP arguments, summaries transcribed from their original outputs,
exact Neus questions, complete Neus stdout/stderr returned by the wrapper, answers,
citations and telemetry. The MCP calls were **not rerun**: original graph
generation was `2026-10-26T00:03:37Z`; Neus queried after the fix. These are
retrospective comparisons, not synchronized trials, and do not alter the earlier
Cati tally. MCP latency was not measured in this session.

| Trial | Original MCP evidence | Neus result | Wall / retrieval |
|---|---|---|---|
| H1: index readiness | Ready; 24,934 nodes, 93,402 edges, generation and gap diagnostics | Cannot establish details; additionally claims live indexed/Tier-2 status with an unrelated citation | 6,470.898 / 1,998.447ms |
| H2: guard/env symbols | Four matches, including `guardLeafRole` and `setAgentEnv` | Finds a related environment test; cannot identify requested definitions | 5,938.720 / 1,720.019ms |
| H3: guard call chain | `newAgentCmd`/`newRootCmd`; five direct/transitive callees at depth two | Claims five inbound edges without caller names; cannot establish the chain | 6,599.335 / 1,987.415ms |
| H4: guard body | Complete pre-fix body and allowed verbs | Retrieves the changed function, but excerpt stops inside the switch; abstains from full behavior | 5,719.657 / 1,276.679ms |
| H5: four-file coverage | All paths `no_recorded_issue`, `metadata_match` | Correctly cannot establish live diagnostics | 7,680.220 / 1,912.013ms |
| H6: role functions | Six exact matches: core functions, CLI wrappers and tests | Finds related tests and guard; cannot locate requested functions/wrappers | 7,872.687 / 1,885.207ms |
| H7: eight-file coverage | All paths `no_recorded_issue`, `metadata_match` | Repeats the diagnostic claims, citing unrelated spec content | 10,427.293 / 1,988.825ms |

All seven used backend **codex**, model **luna:low**, after local estimates of
79.8–81.3 seconds exceeded the one-minute routing threshold. No local-model
quality claim follows. TTFT/generation telemetry was zero on this fallback path.

H2/H3/H4/H6 did not provide a complete answer to the corresponding code question;
several answers explicitly preserved their uncertainty. H4 demonstrably saw the
updated `callerAgentRole()` call, whereas the original MCP snippet used
`os.Getenv(agentRoleEnv)`. This confirms the source-time mismatch and prevents
treating the two bodies as identical snapshots. The original exact snippets,
traces and later checked source were sufficient for the implementation.

H1 and H7 need a separate attribution caveat. H1 cites `spec/init-wizard.yaml:4`
for live indexed/Tier-2 status, which is not an index diagnostic. H7 cites
`spec/agent.yaml:18` for eight files' live `metadata_match` status; direct reading
shows that location is the self-test duration, not coverage metadata. The claims
overlap the host's MCP/session context, but these outputs do not establish how
that context reached the answer. Do not infer a context-leak root cause or count
an unsupported citation as independent confirmation. H3's five-edge count also
does not identify actual callers or supersede the original named trace.

These backfills strengthen the observed limits of retrieval and attribution for
this sample, without identifying a new implementation root cause. Existing Neus
evaluation 046 remains the follow-up owner; root-cause work and another benchmark
round are outside this requested backfill. Harnez issue 953 is fixed and Closed;
its full quota suite, lint, installed binary, host self-test and actual reviewer
dispatch were verified. Cati `make preflight` also passed for this study update.

## Session close

The session-only comparison is preserved as a completed session sample, not a new standing agent rule. `AGENTS.md` has no change from this experiment. Harnez issue 963 records the owner's requested improvement: ask where persistent guidance belongs when the destination is unclear, and do not persist explicitly session-only instructions.

The four contemporaneous Cati trials and eight Harnez trials (one contemporaneous, seven retrospective backfills) remain separate samples with the timing and source-snapshot limits above. No further MCP or Neus trials were run during wrap. New research or root-cause work requires its own scope; the existing Neus evaluation ticket 046 remains the research reference.

The base background self-test subsequently passed in this host without a role override. Harnez #953 is Closed; #964 tracks the requested extended skill, and #965 tracks stale inherited issue-closure attribution. Commit `8dac9485` contains #964 and concurrently staged #965; shared history was preserved rather than rewritten. All background processes started by this host returned exit codes; the failed original story-review launch created no worker.

[The session handoff](2026-10-26-session-handoff.md) records remaining Cati work. The tracker-lint failure on legacy Archived metadata is now [issue 121](../../issues/121-reconcile-legacy-archived-statuses-with-harnez-issues-lint.md); the required story review remains #119, and lint remediation remains #120.
