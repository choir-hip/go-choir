# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities privately through GitHub's private
vulnerability reporting:

1. Open the repository's **Security** tab
   (https://github.com/choir-hip/go-choir/security).
2. Choose **Report a vulnerability** and fill in the advisory form.

Do not open a public issue, pull request or discussion for a suspected
vulnerability, and do not test against other people's computers or data.

Useful details: the affected component and commit, steps to reproduce, the
impact you observed, and whether you believe it is being exploited.

## What happens next

- We acknowledge the report in the advisory thread, confirm or rule out the
  issue, and agree a disclosure date with you.
- Fixes land on `main` and deploy from there; the advisory is published
  once the fix is running.
- Reporters are credited in the advisory unless they ask not to be.

## Scope

In scope: the code in this repository and the service it runs at
https://choir.news. That includes the host services, the guest computer
image (kernel, Firecracker VMM configuration, capsules), the frontend, the
build and deploy workflows in `.github/`, and the Nix flake.

Out of scope here: vulnerabilities in upstream projects (the Linux kernel,
Firecracker, nixpkgs packages, Go, npm packages, model and search
providers). Please report those upstream. If an upstream issue affects
Choir in a way the upstream report would not cover, report that part here.

## Supported versions

Only the current `main` branch and the deployment built from it are
supported. There are no maintained release branches.

## How Choir keeps up with upstream fixes

The patch pipeline and its time-to-patch targets are described in
[docs/archive/reports/patch-pipeline-time-to-patch-2026-10-15.md](docs/archive/reports/patch-pipeline-time-to-patch-2026-10-15.md).
