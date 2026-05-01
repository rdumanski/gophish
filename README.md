![gophish logo](https://raw.github.com/gophish/gophish/master/static/images/gophish_purple.png)

Gophish (rdumanski's fork)
==========================

> **Modernization fork of [Gophish](https://github.com/gophish/gophish) 0.12.1.**
> Upstream is effectively dormant (last release Sep 2024). This fork tracks
> upstream for security fixes only and develops new features on a modernized stack.
>
> - **Module path:** `github.com/rdumanski/gophish`
> - **Current version:** `0.13.0-dev`
> - **Roadmap:** Go 1.22 toolchain, GORM v2, Vite + TypeScript frontend, plugin
>   architecture, then new features (modern phishing techniques, AI/LLM
>   integration, expanded API + integration ecosystem).

![Build Status](https://github.com/rdumanski/gophish/workflows/CI/badge.svg)

## What is Gophish?

[Gophish](https://getgophish.com) is an open-source phishing toolkit designed for businesses and penetration testers. It provides the ability to quickly and easily setup and execute phishing engagements and security awareness training.

## Install

> **Note:** This fork has not yet cut a binary release. Use "Building From Source" below until releases are published.

Once releases are available, you will be able to download and extract a zip containing the [release for your system](https://github.com/rdumanski/gophish/releases/) and run the binary.

## Building From Source

This fork is in active modernization. The Go toolchain requirement will move from 1.13 to 1.22 in the next phase. For now:

```bash
git clone https://github.com/rdumanski/gophish.git
cd gophish
go build
```

After this you should have a binary called `gophish` in the current directory.

## Docker

The Dockerfile in this repository is currently being updated as part of the modernization roadmap. The upstream image at `gophish/gophish` on Docker Hub is **not** maintained by this fork.

## Setup

After running the gophish binary, open a browser to https://localhost:3333 and log in with the default username (`admin`) and the password printed in the log output, e.g.:

```
time="2026-04-30T01:24:08Z" level=info msg="Please login with the username admin and the password 4304d5255378177d"
```

The first-run admin password can be overridden via the `GOPHISH_INITIAL_ADMIN_PASSWORD` environment variable.

## Documentation

Upstream documentation lives at [getgophish.com/documentation](http://getgophish.com/documentation). Fork-specific changes (modernization phases, new APIs, plugin architecture) are documented under `docs/` in this repository.

## Issues

Find a bug? Want more features? Please [file an issue](https://github.com/rdumanski/gophish/issues/new) on this fork. For issues that also affect upstream Gophish, the upstream issue tracker is at [github.com/gophish/gophish/issues](https://github.com/gophish/gophish/issues).

## License

MIT. See [LICENSE](./LICENSE) for the full text.

```
Copyright (c) 2013-2020 Jordan Wright (upstream Gophish)
Copyright (c) 2026 Radek Dumanski (fork modifications)
```
