# Contributing to Choir

Thanks for helping. This file covers how a contribution is licensed and
signed off. For how the codebase is organised and how agents and humans work
in it, read [CLAUDE.md](CLAUDE.md) (the operating contract) and
[docs/choir-doctrine.md](docs/choir-doctrine.md) (the architecture).

## Licence

Choir is licensed under the Apache License, Version 2.0. See
[LICENSE](LICENSE) and [NOTICE](NOTICE). By contributing, you agree that your
contribution is licensed under Apache-2.0, the same as the project. Third-party
components keep their own licences.

## Sign-off: the Developer Certificate of Origin (DCO)

Choir takes outside contributions under a DCO sign-off, not a Contributor
Licence Agreement (CLA). You keep your copyright; you certify that you have the
right to submit the work under the project licence.

The full text is at <https://developercertificate.org/>. In short, by signing
off you certify that you wrote the contribution (or have the right to submit it)
and that you are submitting it under the open source licence of this project.

Sign off every commit with `-s`:

```sh
git commit -s -m "fix: describe the change"
```

This appends a trailer using your configured `user.name` and `user.email`:

```text
Signed-off-by: Your Name <you@example.com>
```

Use your real name and an address you control. To add a missing sign-off:

```sh
git commit --amend -s --no-edit     # the last commit
git rebase --signoff main           # every commit on your branch
git push --force-with-lease         # then update your branch
```

### What the check does

The `DCO` workflow (`.github/workflows/dco.yml`) runs `git log` over the commits
of a pull request and looks for a `Signed-off-by: Name <email>` line on each
non-merge commit. It is required on pull requests from forks. On pushes to
`main` and on pull requests from branches of this repository it only reports
(a warning, never a failure). It uses no secrets and no third-party action, and
it is separate from the main CI and deploy workflow.

## Trademark

The name "Choir" and its logos are not licensed by Apache-2.0 (section 6 of
the licence grants no trademark rights). A trademark policy for the name will
be published later. Until then, please do not use the name in a way that
suggests your fork or product is the official project.

## Before you open a pull request

- Keep the change focused, and say what problem it solves and how you checked it.
- Run the tests for the packages you touched, for example
  `go test ./internal/<pkg> -run TestName`.
- Documentation-only changes are welcome and use a lighter CI path.
