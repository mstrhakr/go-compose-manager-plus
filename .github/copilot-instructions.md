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
- For UI implementation and polish, use
  [refine-ui](skills/refine-ui/SKILL.md): capture real-browser screenshots and
  compare them with references after each coherent set of visual changes,
  iterating before claiming alignment. Keep private reference images private.
- Run focused tests while editing, then `make check` for completed app work.
  Use `make docs`, `make check-workflows`, `make check-security` and container checks
  when their surfaces change. State checks not run; never invent verification.
- Keep [user documentation](../docs/content/docs/) current incrementally as
  features and UI evolve. Add or update pages within relevant feature slices
  to explain available functions, navigation, controls, workflows and limits;
  document implemented behavior, not future plans as if they already work.
  An update is not required for every commit, but do not defer user-facing
  guidance to a large catch-up rewrite. Keep development/internal details in
  development docs or contracts rather than user guides.
- Use Conventional Commit messages for every authored commit and PR title.
  Preserve breaking intent in squash messages. Work through short-lived branches
  and PRs; do not push main, publish, or bypass protection without authorization.
- Finish the current branch's work before starting unrelated work. Keep each
  branch focused on one coherent task; do not accumulate mixed changes or stop
  at a local commit when delivery is requested. Complete validation and follow-up
  fixes, obtain any required authorization, push the branch, open its PR, wait
  for required checks and address review feedback, then merge through normal
  protections. Confirm the merge, sync local main, and clean up the completed
  branch before moving on. If blocked by permissions, checks or an owner decision,
  report the blocker and pause delivery rather than bypassing safeguards or
  silently switching tasks. This does not authorize unrelated work, dependency
  update merges, publishing or protection bypasses.
- Keep PR validation read-only and secret-free. Never execute PR source in a
  privileged workflow. Scope writes/OIDC per job; review pins, executable downloads,
  dependencies and build/release tooling. Do not automatically merge updates.
- Follow the [development guide](../docs/content/docs/development.md) and
  [delivery guide](../docs/content/docs/deployment.md). Keep native-platform evidence
  separate from cross-compilation and simulation.