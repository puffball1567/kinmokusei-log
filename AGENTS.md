# Repository instructions

Read [CONTRIBUTING.md](CONTRIBUTING.md) before making changes.

- Create focused branches from current `devel`; ordinary pull requests target
  `devel`. Complete integration through the remote pull request, then update
  local `devel`; never substitute a local merge or direct protected-branch push.
- Promote releases from `devel` to `main` through a remote merge-commit pull
  request only after CI succeeds for the exact current `devel` HEAD.
- Use `hotfix/*` from `main` only for urgent release fixes, run full CI, and
  synchronize `main` back to `devel` through a pull request afterward.
- Create version tags only from the merged `main` state.
- Preserve existing project validation and run the branch-flow self-test when
  changing workflows or branch policies.
- Public pull requests and release notes contain implementation changes and
  concise validation results only. Do not include credentials, machine-specific
  paths, or internal operational and publishing instructions.
