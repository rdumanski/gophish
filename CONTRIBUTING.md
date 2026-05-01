# Contributing

Thank you for your interest in contributing to **rdumanski's fork of Gophish**.

This document describes the contribution model, branch strategy, and how this
fork relates to upstream `gophish/gophish`.

## Project status

This repository is a **fork of Gophish 0.12.1** under active modernization. Upstream
is effectively dormant (last commit September 2024). The fork is being prepared
for new feature development on a modernized stack — see [README.md](./README.md)
for the high-level roadmap.

The fork's posture toward upstream:

- **No automatic rebases.** Upstream is not regularly merged into the fork because
  the fork's modernization (Go 1.22, GORM v2, Vite frontend, plugin architecture)
  diverges substantially from upstream's structure.
- **Security fixes are cherry-picked.** When upstream lands a security fix that
  applies to the fork, it is cherry-picked from `upstream/master`. The cherry-pick
  is recorded in the commit message: `cherry-picked from upstream <sha>`.
- **Upstream tracking remote.** The repository has an `upstream` remote pointing
  at `https://github.com/gophish/gophish`. Maintainers periodically fetch and
  review for security-relevant changes.

## Branch model

- `main` is the default branch and the source of truth.
- Feature work happens on **feature branches** named `phase-N-<topic>` or
  `feat/<topic>`. PRs target `main`.
- Releases are tagged `vMAJOR.MINOR.PATCH` (semver, starting from `0.13.0`).
  Pre-releases use `-alpha.N` / `-beta.N` / `-rc.N` suffixes.
- Merges are **squash-merged** — every PR is one commit on `main`.
- Conventional commit prefixes are encouraged but not enforced (`feat:`, `fix:`,
  `chore:`, `refactor:`, `docs:`, `test:`).

## Development environment

The Go toolchain version, Node version, and build prerequisites will be documented
in `docs/dev/build-*.md` as part of Phase 1 of the modernization roadmap. Until
then, the build prerequisites match upstream Gophish 0.12.1 (Go 1.13+, Node 12+).

## Submitting a pull request

1. Fork this repository (the fork-of-a-fork pattern).
2. Create a feature branch from `main`.
3. Make your changes. Run `go test ./...` and ensure the smoke test (admin login,
   send test campaign) still works.
4. Open a PR against `main`. Describe the change and link to any relevant issue.
5. Be patient — this is a solo-maintained project.

## Reporting a security vulnerability

See [SECURITY.md](./SECURITY.md). **Do not** open a public issue for security
matters.

## Reporting a bug or requesting a feature

Use [GitHub Issues](https://github.com/rdumanski/gophish/issues). A bug report
should include:

- Affected version (commit SHA or tag).
- Reproduction steps.
- Expected vs actual behavior.
- Logs or stack traces if available.

A feature request should describe the use case and how the feature fits into the
project's roadmap.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](./LICENSE), the same license under which the project is distributed.
There is no separate Contributor License Agreement.

## Acknowledgement

This fork builds on the work of **Jordan Wright** and the upstream Gophish community.
The upstream codebase remains MIT-licensed and its copyright is preserved in all
derivative work.
