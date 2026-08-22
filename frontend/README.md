# S-UI-Frontend
** A frontend for S-UI **

[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Disclaimer:** This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment

## Project setup

```
# yarn
yarn

# npm
npm install

# pnpm
pnpm install

# bun
bun install
```

### Compiles and hot-reloads for development

```
# yarn
yarn dev

# npm
npm run dev

# pnpm
pnpm dev

# bun
pnpm run dev
```

### Compiles and minifies for production

```
# yarn
yarn build

# npm
npm run build

# pnpm
pnpm build

# bun
pnpm run build
```

## Independent deployment

`npm run build` creates a standalone SPA in `dist/`. Serve that directory with
any static web server and route unknown frontend paths back to `index.html`.
The asset URLs are relative, so the same build can be mounted at `/` or a
subpath such as `/app/`.

Edit the unhashed `dist/.well-known/1s-ui/config.js` after deployment; rebuilding
is not needed. The static server must expose that file at the fixed same-origin
URL `/.well-known/1s-ui/config.js`, including when the SPA is mounted below a
subpath:

```js
window.__SUI_CONFIG__ = {
  backendUrl: 'https://api.example.com', // empty means the frontend origin
  basePath: '/app/',                     // frontend mount path
}
```

`window.__SUI_CONFIG__` takes priority over `VITE_BACKEND_URL` and
`VITE_BASE_PATH`; their defaults are an empty backend URL and `/`. When the
frontend and backend use different origins, the backend must allow the exact
frontend origin with credentials and issue a compatible session cookie.

During development, Vite proxies `/api`, `/apiv2`, and `/agent/v1` to
`VITE_DEV_PROXY_TARGET` (default `http://127.0.0.1:2097`).

### Type checks

```
# yarn
yarn lint

# npm
npm run lint

# pnpm
pnpm lint

# bun
bun run lint
```
