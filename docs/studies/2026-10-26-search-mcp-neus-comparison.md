---
kind: topic
title: "Session Study: Codebase Memory MCP versus neus ask"
date: 2026-10-26
status: ongoing
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
