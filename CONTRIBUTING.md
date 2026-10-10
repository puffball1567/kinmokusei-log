# Contributing to kinmokusei-log

## Branch and release workflow

- `devel` is the default development branch; `main` contains released code.
- Start feature and fix branches from the current `devel`, and open pull requests
  against `devel`. Integrate through GitHub pull requests, not local merges or
  direct pushes to protected branches, then synchronize your local `devel`.
- Keep `devel` green. Development pull requests and pushes to `devel` run the
  existing full CI suite. The required `test` check fails if a required job fails
  or is unexpectedly skipped.
- Release through a separate pull request from this repository's `devel` to
  `main`. The release check requires the exact current `devel` HEAD to have a
  successful push run of `.github/workflows/ci.yml`, reusing that tested result
  instead of repeating the full suite.
- Merge release pull requests with a **merge commit**. Do not squash or rebase:
  preserving ancestry keeps subsequent releases and synchronization consistent.
- Create version tags from `main` only after the release pull request is merged.
  Version tags run the full CI suite.
- For an urgent released-code correction, create `hotfix/*` from `main` and open
  a pull request to `main`. Hotfixes run the full CI suite and use a merge commit.
  Afterward, synchronize `main` back into `devel` through another pull request,
  which runs the full suite.
- Pull requests to `main` must originate from this repository's `devel` or
  `hotfix/*`. Forks contribute through `devel`; fork branches cannot promote
  directly to `main`.
- Protected branches require pull requests, successful required checks, resolved
  review conversations, and protection against deletion and force pushes.
  `devel` also requires one approval and approval of the latest push; `main`
  accepts merge commits only and requires `Require devel source` in addition to
  `test`. Administrative bypass permissions do not replace this contribution flow.

## Validation

Follow the toolchain requirements in [README.md](README.md). Run the branch
policy self-test and the existing project tests before opening a pull request:

```sh
sh scripts/branch_flow.sh self-test
sh scripts/test.sh
```

Keep pull request descriptions focused on behavior changes and validation
results. Explain compatibility changes and update affected examples and docs.

## Contribution license

Unless explicitly stated otherwise, contributions are provided under the
[Apache License 2.0](LICENSE). Submit only work you have the right to license.
