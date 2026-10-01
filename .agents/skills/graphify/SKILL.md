---
name: graphify
description: Teach agents to use Graphify as CURRA's structural repository intelligence layer.
---

# Purpose
To efficiently query repository structure, module connections, and blast radius without reading every file. Graphify answers "How is the code connected?"

# Use When
Investigating dependencies, determining the impact of a change, or tracing data flow between layers (e.g., frontend to backend).

# Required Context
- `graphify/GRAPH_REPORT.md`
- `graphify/README.md`

# Workflow
1. **Query Graphify First**: Use the generated Graphify report for structural questions before manual searches.
2. **Narrow Scope**: Identify the relevant subgraph for your task.
3. **Validate**: Inspect the actual source code to validate Graphify's conclusions.
4. **Reference Analysis**: Check dependency paths before determining a piece of code is dead or isolated.

# Verification
Ensure you can answer:
- Where is this symbol used?
- What depends on this package?
- What is the dependency path from frontend to backend?
- What is the blast radius of changing this type?
- Is this service/repository/handler connected to the actual application flow?
- Which parts of the solver are affected by this change?
- Why does this node appear isolated?

# Guardrails
- Graphify is **structural memory**. It does NOT replace reading authoritative source code, contracts, CURRA_CONTEXT.md, or verifying runtime behavior.
- **NEVER** dump massive Graphify raw outputs into context unnecessarily.
- Regenerate/refresh Graphify only when the architecture changes materially.
- Treat committed Graphify artifacts as a structural snapshot, not the ultimate source of truth.

# Related Skills
- Use `repo-audit` to begin exploration.
- Use `architecture-review` to ensure dependencies respect constraints.
- Use `solver-change` when modifying core areas identified by Graphify.
