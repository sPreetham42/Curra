---
name: testing-hardening
description: Define CURRA's quality-oriented verification workflow.
---

# Purpose
To ensure code changes are rigorously verified using the appropriate hierarchy of tests, protecting system invariants. A passing test suite is necessary but not automatically sufficient.

# Use When
Writing tests for new functionality, fixing bugs, or hardening the solver core.

# Required Context
- The CURRA testing hierarchy: `unit` → `invariant/regression` → `integration` → `pathological/stress` → `fuzz/red-team` → `full suite`.

# Workflow
1. **Identify Risks**: Ask "What invariant could this change break?"
2. **Select Level**: Choose the appropriate level of verification from the testing hierarchy (use only the levels relevant to the change).
3. **Implement Test**: Write the test focusing on the core invariants.
4. **Execute**: Run targeted tests, then verify against the broader suite if necessary.

# Verification
Special attention should be paid to:
- Determinism and seeded PRNG stability.
- Cancellation and deadline enforcement.
- Node limit boundaries.
- `SolutionIndex` consistency and `RuleSetHash` stability.
- Transactional move/swap rollback behavior.
- Verifier independence.
- Locked assignments.
- Hard constraint preservation.
- Panic resistance for invalid inputs.

# Guardrails
- **DO NOT** write flaky timing tests or arbitrary sleep-based synchronization.
- **DO NOT** write tests that depend on map iteration order.
- **DO NOT** write tests that validate implementation details unnecessarily.
- **NEVER** weaken assertions simply to make tests pass.

# Related Skills
- Use `solver-change` when hardening core components.
- Use `api-contract` for integration level testing.
