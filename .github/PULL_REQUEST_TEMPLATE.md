<!--
Thanks for contributing! Please fill out this template so the change is easy
to review. PRs that don't follow the template may be asked to revise.
-->

## What does this PR do?

<!-- Brief description of the change. -->

## Why?

<!-- Motivation, linked issue, roadmap phase. -->

## How was it tested?

- [ ] `go build ./...` clean
- [ ] `go test ./...` passes
- [ ] Smoke test: admin login + send test campaign + open/click tracking still works
- [ ] (If touching frontend) `npm run build` succeeds and the affected page works in a browser
- [ ] (If touching DB schema) migrations apply cleanly on a fresh sqlite DB AND a snapshot of an existing 0.12.1 DB

## Roadmap phase

<!-- e.g. Phase 0 fork housekeeping, Phase 2 Go modernization, etc. -->

## Notes for reviewers

<!-- Anything tricky, follow-up tasks, or things you specifically want feedback on. -->

## Checklist

- [ ] Conventional commit prefix on the squash-merge title (`feat:`, `fix:`, `chore:`, `refactor:`, `docs:`, `test:`)
- [ ] No new lint findings beyond `docs/dev/lint-debt.md` exemptions
- [ ] No secrets, API keys, or production credentials in the diff
