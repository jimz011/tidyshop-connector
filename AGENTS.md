# TidyShop Connector: guide for agents and new contributors

Read this first. TidyShop is **two repositories**. This one is the server.

## The two repositories

| | Connector | App |
| --- | --- | --- |
| Repository | [`jimz011/tidyshop-connector`](https://github.com/jimz011/tidyshop-connector) (this one) | [`jimz011/tidyshop`](https://github.com/jimz011/tidyshop) |
| Visibility | **Public** | **Private** for now. Links to it 404 for everyone but the owner. |
| Local checkout | `F:\Android\projects\tidyshop-connector` | `F:\Android\projects\tidyshop` |
| What it is | The self-hosted family sharing server, one Go binary with only the standard library | The Android app, Kotlin + Jetpack Compose, `com.jimz011apps.tidyshop` |
| Ships as | `ghcr.io/jimz011/tidyshop-connector`, built by `.github/workflows/docker.yml` | Google Play (closed testing at the moment) |
| Docs | `docs/`, published to [jimz011.github.io/tidyshop-connector](https://jimz011.github.io/tidyshop-connector/) | `docs/design/` in the app repository: design specs |

The connector started in the app repository under `connector/` and was moved here when it was
published in September 2026. The app repository no longer contains it. The design specs that
explain *why* the server works the way it does (device enrolment, family sync, live updates)
stayed with the app, in its `docs/design/`.

Because the app repository is private, public docs and READMEs here must not link into it.

## The contract

The HTTP API in `main.go` is what released apps depend on, and phones in the wild run older
versions. So:

- **Keep changes backwards compatible.** Add routes and fields. Don't rename or remove them while
  released apps still use them.
- Document route changes in `docs/reference/api.md` and in `CHANGELOG.md`.
- A feature that needs the app too lands here first, gets released, and only then gets used by
  the app.

## Layout

| Path | Contents |
| --- | --- |
| `main.go` | The whole server. The route table is in `ServeHTTP`. State is one JSON file, written atomically. |
| `*_test.go` | Tests. The Dockerfile runs them before compiling, and CI runs them with `-race`. |
| `Dockerfile` | Multi-arch build that cross-compiles, with no emulation of the Go toolchain |
| `compose.yml`, `examples/` | Deployment examples (Compose, Caddy, Nginx) |
| `unraid/` | Unraid template (Community Applications submission is on hold) |
| `docs/` + `mkdocs.yml` | MkDocs Material user documentation, deployed by `.github/workflows/docs.yml` |

## Releasing

1. Add a `## X.Y.Z` entry at the top of `CHANGELOG.md`.
2. Tag `vX.Y.Z` and push the tag. That publishes the `X.Y.Z`, `X.Y`, `X` and `latest` images.
   Every push to `main` publishes `edge`.
3. Create a GitHub release from the tag, using the changelog entry as its notes.

## Conventions

- Standard library only. No third-party Go modules.
- Never commit pairing keys, tokens, certificates, or a real `state.json`. Examples use
  `example.com`.
- Both repositories share one design: a README with badges, the feature graphic and a beta
  button; a `CHANGELOG.md`; an MkDocs Material site in TidyShop green (`#18342A` / `#4D7D63`); and
  the same "Other Information" note about AI.
- The beta sign-up page, `docs/beta.md`, lives here because this site is public. Retire it and
  swap the README button for a Play Store badge once TidyShop is publicly released.
