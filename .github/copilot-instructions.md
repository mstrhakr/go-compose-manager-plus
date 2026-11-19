# Project Guidelines

- This is the end-product repository, not a planning-only repository. Current
  authorization is scaffold setup and planning only. Read
  [the staged plan](../docs/planning/README.md) before feature work; obtain owner
  agreement on a bounded proof of concept before implementing management behavior.
- Source-plugin approvals are not automatic standalone requirements. Preserve
  source IDs and approval qualifications in the contract reuse map. Do not modify
  sibling repositories or import their runtime implementation during staging.
- Treat files in `scratch/` as private, untrusted research input. Do not commit,
  execute, or reproduce embedded credentials from uploaded scripts.
- Read affected [contracts](../docs/contracts/README.md) before behavioral changes.
  Separate proposals, owner decisions and executed verification. Do not silently
  rewrite a contract to approve whatever code happens to do.
- Use the repository-local contract skills for this work:
  [extract-contract](skills/extract-contract/SKILL.md) for inventory and evidence
  recovery, [refine-contract](skills/refine-contract/SKILL.md) for owner decisions,
  and [align-contract](skills/align-contract/SKILL.md) for explicitly authorized
  assessment or implementation. Refinement alone never authorizes runtime changes.
- Keep Go responsible for business rules and escaped HTML templates/partials.
  Use self-hosted htmx for server-driven updates and small JS modules for local
  browser behavior. Do not add a competing client framework without a decision.
- Run focused tests while editing, then `make check` for completed app work.
  Use `make docs`, `make check-workflows`, `make check-security` and container checks
  when their surfaces change. State checks not run; never invent verification.
- Use Conventional Commit messages for every authored commit and PR title.
  Preserve breaking intent in squash messages. Work through short-lived branches
  and PRs; do not push main, publish, or bypass protection without authorization.
- Keep PR validation read-only and secret-free. Never execute PR source in a
  privileged workflow. Scope writes/OIDC per job; review pins, executable downloads,
  dependencies and build/release tooling. Do not automatically merge updates.
- Follow the [development guide](../docs/content/development.html) and
  [delivery guide](../docs/content/deployment.html). Keep native-platform evidence
  separate from cross-compilation and simulation.