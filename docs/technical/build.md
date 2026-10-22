# Building

KOsync needs two tools: **Go** for the server and **Bun** for the web interface. Nothing else.

## The whole thing

```bash
cd server
go run build.go
```

This builds the web interface with Bun, embeds it into the Go binary and writes `./kosync`.

Options:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-web` | `true` | build the web interface first; `-web=false` reuses what is already embedded |
| `-out` | `./kosync` | name of the produced executable |
| `-go` | `go` | path to the Go toolchain |
| `-run` | `false` | start the server after building |

## Only the server

```bash
cd server
go build -tags netgo -o kosync .
```

The web interface is embedded from `internal/webui/public`. When that directory holds nothing but its
`.keep` placeholder, the binary runs fine but serves no interface, and says so in the log.

## Only the web interface

```bash
cd server
go generate ./internal/webui
```

This runs `bun install` and `bun build-only` and writes the result into `internal/webui/public`.

## Developing

Run the server and the Vite dev server side by side:

```bash
cd server && go run . serve            # http://127.0.0.1:8090
cd webui  && bun run dev               # http://127.0.0.1:5173, proxies /api to the server
```

## Tests

```bash
cd server && go test ./...
cd webui  && bun run test
```

## Pinned transitive packages

`webui/package.json` carries a `resolutions` block. JSON has no comments, so the reasoning lives
here:

| Entry | Why |
| --- | --- |
| `minimatch/brace-expansion: ^5.0.11` | `minimatch@10` asks for `^5.0.8`, which still admits the releases carrying GHSA-qhr7-859c-m2p7 and GHSA-6j4f-fj2g-mc7p (fixed in 5.0.11) and the older GHSA-rgw5-rvv9-x895. Only that path is pinned, so a future 2.x copy pulled in by something else is not forced across a major version. |

It is a build and development dependency and never ends up in the browser bundle. Check with
`bun audit` after changing dependencies, and drop the entry once `minimatch` ships a fixed range on
its own.

A nested resolution — `parent/child` — needs **Bun 1.4 or newer**. Bun 1.3 does not support them:
it prints a warning, ignores the entry, and resolves the tree as if it were not there. A lockfile
written by one of the two is therefore not `--frozen-lockfile` clean for the other, which is why CI,
the Dockerfiles and local development all have to use the same Bun version (`BUN_VERSION` in
`.gitlab-ci.yml`).

There is deliberately no `bun.lockb`: this project uses the text `bun.lock`, and having both makes
`bun install --frozen-lockfile` depend on the Bun version.

## One thing that needs Node

`bun run type-check` runs `vue-tsc`, which type checks `.vue` files by patching the TypeScript
compiler through `fs.readFileSync` while it is being required. Bun's module loader does not read
modules through that API, so under Bun the patch never applies: `vue-tsc` quietly falls back to
plain `tsc`, stops understanding `.vue` files altogether, and reports every `.vue` import as
"Cannot find module".

So the type check needs `node` on the PATH. Everything else — `bun run test`, the linters, the vite
build, and therefore `go generate ./internal/webui` — works with Bun alone.

If you ever see a wall of `TS2307: Cannot find module './App.vue'`, that is this and not a broken
import. Do not "fix" it by adding a `declare module '*.vue'` shim: that would silence the real type
checking of every component.

The Go tests create a throwaway PocketBase instance per test and run the real migrations against it,
so they cover the schema and its access rules as well as the handlers.

## Docker

```bash
docker build -f deployment/Dockerfile -t kosync .
docker run -v ./data:/pb_data -e ENABLE_WEBUI=true -p 8080:8080 kosync
```
