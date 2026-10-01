---
name: repo-audit
description: Teach an agent how to enter CURRA without wasting context on unnecessary repository-wide exploration.
---

# Purpose
To safely orient an agent inside the CURRA repository efficiently without unnecessary broad scanning or destroying existing work.

# Use When
Starting a new task, entering the repository for the first time, or establishing the current state before making changes.

# Required Context
- `CURRA_CONTEXT.md`
- `graphify/GRAPH_REPORT.md` (Structural Context)

# Workflow
1. **Check Status**: Run `git status` to identify active/uncommitted changes.
2. **Contextualize**: Read CURRA_CONTEXT.md.
3. **Analyze Structure**: Use Graphify to identify the affected subsystem.
4. **Targeted Inspection**: Inspect only the targeted source code relevant to the task boundary.
5. **Establish Baseline**: Run existing tests on the target component before modifying any code.
6. **Limit Scope**: Work only within the relevant scope identified.

# Verification
Answer the following questions before executing code changes:
- What is the task boundary?
- Which package/layer owns the behavior?
- What depends on it, and what does it depend on?
- What tests already cover it?
- Is the requested change crossing an architectural boundary?

# Guardrails
- **NEVER** begin by reading the entire repository.
- **NEVER** assume the working tree is clean.
- **NEVER** discard or overwrite existing developer/agent work.
- Use targeted inspection over broad `grep`/search.
- Distinguish current actual implementation from assumptions or documentation.
- Detect generated/temp/local artifacts but do not delete ambiguous files.
- Preserve unrelated work.

# Related Skills
- Use `graphify` for structural questions.
- Use `architecture-review` to check boundaries.
- Use `testing-hardening` to verify the baseline.
