---
name: git-checkpoint
description: Make Git history show meaningful engineering milestones.
---

# Purpose
To maintain a high-quality Git history by committing coherent semantic milestones rather than arbitrary file batches.

# Use When
A feature foundation is complete, an architectural boundary is established, a hardening milestone is complete, an API contract milestone is complete, a frontend milestone is complete, or a production-readiness milestone is complete.

# Required Context
- `git status` and `git diff`

# Workflow
1. **Status**: Run `git status`.
2. **Review Diff**: Inspect the diff (`git diff`) and check for whitespace/errors (`git diff --check`).
3. **Verify Tests/Build**: Ensure the test suite and build pass.
4. **Clean Tree**: Verify no generated artifacts, local secrets, or unrelated changes are present.
5. **Stage Intentionally**: Add only the relevant files.
6. **Final Review**: Inspect the staged diff (`git diff --cached`).
7. **Commit**: Write a meaningful semantic commit message.

# Verification
- The commit represents a logical, coherent milestone.
- Message format uses semantic prefixes (e.g., `feat: establish resource management foundations`, `fix: make cancellation tests deterministic`, `chore: publish repository graph context`).
- Final working tree state is verified after committing.

# Guardrails
- **NEVER** overwrite another developer's work.
- **NEVER** silently reset/discard changes.
- **DO NOT** stage unrelated modifications.
- **DO NOT** commit generated Graphify logs/caches or local secrets.
- **AVOID** lazy commit messages like "update", "changes", "fix", or "WIP".
- This skill recommends a checkpoint commit, but **MUST NOT** automatically commit unless the task explicitly authorizes committing.

# Related Skills
- Use `repo-audit` to establish a clean working state.
- Use `testing-hardening` to ensure the milestone is stable before committing.
