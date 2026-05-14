# Choir Lifecycle Load Dynamics

These k6 scripts exercise deployed product-path surfaces without browser-public
internal routes.

Common environment:

```sh
export CHOIR_BASE_URL=https://draft.choir-ip.com
```

For authenticated bootstrap load, first create a Playwright storage state:

```sh
cd frontend
pnpm auth:setup -- --base-url "$CHOIR_BASE_URL"
export CHOIR_AUTH_STATE="$PWD/playwright/.auth/$(node -e "console.log(new URL(process.env.CHOIR_BASE_URL).hostname.replaceAll('.', '-'))").storage.json"
cd ..
```

Examples:

```sh
k6 run load/k6/public-progressive.js
k6 run load/k6/public-stochastic.js
k6 run load/k6/authenticated-bootstrap-progressive.js
```

The scripts intentionally use only public root, `/health`, `/auth/session`, and
authenticated `/api/shell/bootstrap`. They do not call `/internal/*`,
`/api/test/*`, `/api/agent/*`, or raw event mutation endpoints.
