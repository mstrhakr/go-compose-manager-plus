# Product Staging Verification

Date: 2026-11-19. Environment: Linux development container. Scope: initial
product scaffold and planning, imported from published go-template main
`f281859b8c912ab7e251c7b2a6ddcaf6e663393b`. This record applies to the bootstrap
commit containing it; it does not inherit the template's release evidence.

## Executed

- Initial imported scaffold: `make check-go` passed vet, Go race tests, formatting
  check and build.
- After module rename: first `make check-go` caught a test import still using
  go-template; corrected it, then the same command passed all three Go packages.
- `npm ci --ignore-scripts`: passed; install audit reported zero vulnerabilities.
- `make test-unit check-contracts`: passed 14 Node tests and structure/stable-ID
  checks for two inherited contracts. This does not approve their behavior.
- `git check-ignore scratch/manager.sh`: confirmed uploaded script files ignored.
- Planning/instruction relative-link check and `git diff --check`: passed.

- Final `make check`: passed Go vet/race tests/format/build, 14 Node unit tests,
  6 API tests, 24 desktop/mobile browser tests, embedded-asset verification and
  both inherited contract structure/stable-ID checks.
- `make check-workflows`: passed workflow policy checks and Actionlint.
- `make check-security`: npm audit and govulncheck reported no vulnerabilities.
- `make docs`: passed the default Hugo build with warnings fatal.
- Contract-inclusive Hugo build with `hugo.toml,hugo.contracts.toml`: passed
  with warnings fatal.
- GitHub readback: public visibility; active main/version-tag rulesets;
  secret scanning, push protection and Dependabot security updates enabled;
  SHA-pinned selected Actions with read-only default tokens; squash-only merges;
  release approval configured. Release, container, docs and contract publishing
  variables are explicitly false.

## Not Verified By These Checks

No Compose SDK installed or built; no Docker management behavior exists.
No source-plugin test suites were run. No representative Ubuntu VM measurement,
resource budget, auth, persistence, lifecycle/restart recovery or network/volume
mutation guarantee is verified. Script inspection remains pending.

Native hosted Windows/macOS runs, container runtime, image scans, published
downloads/images/docs, signing, release approvals and consumer attestation
verification are not implied by template imports or local Linux tests.
The owner approved public visibility on 2026-11-19. The public-only GitHub setup
helper completed successfully, with readback confirming active protected-main
and immutable-version-tag rulesets, selected SHA-pinned Actions, disabled
release/container publishing and a release environment requiring owner approval.
This settings readback is not evidence that hosted workflows have run.