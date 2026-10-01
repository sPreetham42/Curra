---
name: solver-change
description: A specialized safety procedure for touching the CURRA core scheduler.
---

# Purpose
To strictly govern modifications to the frozen solver core (`internal/scheduler`), protecting feasibility, determinism, and optimization invariants. This is one of the most important CURRA skills.

# Use When
Modifying constraints, heuristics, solver components, or the solution index.

# Required Context
- `CURRA_CONTEXT.md` (Solver domain rules)
- `internal/scheduler` source code

# Workflow
1. **Contextualize**: Read the relevant CURRA_CONTEXT.md sections.
2. **Analyze**: Use Graphify to understand the affected solver subgraph.
3. **Identify Invariants**: Determine which invariants (e.g., determinism, index consistency) are affected.
4. **Baseline**: Establish the relevant test baseline.
5. **Implement**: Make the *smallest* necessary change.
6. **Test**: Add regression tests where behavior changes.
7. **Verify Determinism**: Run targeted determinism checks.
8. **Verify Core Guarantees**: Ensure hard constraints remain inviolable, context cancellation/deadlines work, and independent solution verification passes.
9. **Finalize**: Run targeted tests, then broader regression tests.

# Verification
Explicitly preserve:
- CSP logic handles hard-constraint feasibility only.
- Tabu Search handles soft-objective optimization only.
- Authoritative verifier remains independent.
- Deterministic map handling/order and seeded randomness.
- Context cancellation.
- Locked assignment preservation.
- `SolutionIndex` consistency.
- Root solver module remains dependency-free.

# Guardrails
- **DO NOT** add product features while doing solver work.
- **DO NOT** move optimization logic into CSP.
- **DO NOT** allow Tabu Search to tolerate hard violations.
- **DO NOT** bypass independent verification.
- **DO NOT** introduce uncontrolled randomness.
- **DO NOT** add third-party dependencies to the root solver module.
- **DO NOT** change the frozen boundary without an explicit architectural decision.

# Related Skills
- Use `graphify` to understand solver impacts.
- Use `architecture-review` to ensure changes don't leak out.
- Use `testing-hardening` for rigorous verification.
- Use `git-checkpoint` after successful verification.
