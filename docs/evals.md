# 📊 Evaluation Framework Guide (Level 1 Evals)

This document explains **Level 1 evaluations** for the AI SRE project—simple, truth-valued assertions that help us catch regressions and measure improvements over time.

---

## 1 — Why Level 1 Evals?

| Goal | How Level 1 Evals Help |
|------|-----------------------|
| **Spot failures early** | Automated assertions flag obvious mistakes (missing flags, wrong tool, etc.) |
| **Focus doc work** | Failures pinpoint gaps in reference docs, guiding us on what to improve. |
| **Measure progress** | A stable test set + pass-rate stats let us quantify how doc or model tweaks affect quality. |





## 2 — Core Concepts

| Concept | Proto Message | Description |
|---------|---------------|-------------|
| **EvalDataset** | `EvalDataset` | A collection of `EvalSample`s plus metadata. |
| **EvalSample** | `EvalSample` | One **input document** + a list of **Assertion**s to check. |
| **Assertion** | `Assertion` | A single check that yields **PASS / FAIL / SKIPPED**. |
| **Assertor** (Go) | N/A | Concrete implementation that evaluates one `Assertion` type. |

### Supported Assertion Kinds (v0)

| Kind | Assertor | Status |
|------|----------|--------|
| `ShellRequiredFlag` | `shell_required_flag_assertor.go` | ✅ Implemented |
| `ToolInvocation` | `tool_invocation_assertor.go` | 🛠️ Stub |
| `FileRetrieval` | `file_retrieval_assertor.go` | 🛠️ Stub |
| `LlmJudge` | `llm_judge_assertor.go` | 🛠️ Stub |

---

## 3 — Quick Start

```bash
# Build the CLI
make build

# Run the eval against a local Cassie endpoint
./.build/cas eval dataset.pb http://localhost:8080