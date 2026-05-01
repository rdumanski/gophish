# Build on Windows (native, without Docker)

Building natively on Windows is **only useful for active development** (hot reload via `air`). For one-off builds and CI the [Docker path](./build-docker.md) is simpler.

## Prerequisites

| Tool | Version verified | Install |
|---|---|---|
| Go | 1.26.2 (works with go.mod's `go 1.13`; Phase 2 will bump go.mod) | `winget install GoLang.Go` |
| Node.js | LTS (≥ 18) | `winget install OpenJS.NodeJS.LTS` |
| C compiler (CGO) | gcc 13+ via MSYS2 mingw-w64 | `winget install MSYS2.MSYS2`, then `pacman -S mingw-w64-x86_64-gcc` |
| Git Bash | shipped with Git | `winget install Git.Git` |
| Task runner (optional) | go-task v3 | `winget install Task.Task` |
| Air (optional, hot reload) | latest | `go install github.com/air-verse/air@latest` |

### Why a C compiler?

The default sqlite driver `mattn/go-sqlite3` requires CGO. Without `gcc` on `PATH`, `go build` fails with:

```
undefined: sqlite3.Error
```

(because `error.go` is gated behind `import "C"` and won't compile without a C toolchain).

After installing MSYS2, add the mingw-w64 bin directory to your shell `PATH`:

```bash
# In ~/.bashrc (Git Bash) or ~/.zshrc:
export PATH="/c/msys64/mingw64/bin:$PATH"
```

Verify:

```bash
gcc --version   # should report 13.x or newer
go env CGO_ENABLED  # should report 1
```

> Phase 2 evaluates switching to `modernc.org/sqlite` (pure Go, no CGO). If that migration succeeds, the C compiler requirement disappears.

## Build steps

```bash
# In Git Bash with gcc on PATH and Go installed:
cd C:/Users/radek/dev/gophish
go mod tidy            # one-time, after a fresh clone
go build ./...         # compile-check every package
go build -o gophish.exe .   # produce the binary
```

Or via Taskfile:

```bash
task build      # produces gophish.exe
task build-all  # compile-check every package
```

## Run

```bash
./gophish.exe
```

The first-run admin password is printed to the log. Open `https://localhost:3333`.

## Frontend assets

The `static/js/dist/` and `static/css/dist/` directories must be populated before `gophish` will serve the admin UI. To build them:

```bash
npm install
npx gulp
```

Or:

```bash
task frontend
```

These are committed to `.gitignore`-excluded paths, so a fresh clone has no `dist/` directory until you run the frontend build.

## Hot-reload dev loop

```bash
task dev   # runs `air` which watches .go files and rebuilds + restarts on save
```

The frontend build is not watched by `air`. For frontend hot reload (post-Phase 4 with Vite) the dev loop will be `task dev` for the Go server and a separate `npm run dev` for Vite.

## Known issues on Windows

- **CRLF line endings**: Git's `core.autocrlf` may convert LF to CRLF in `.go` files. The Go toolchain handles this transparently, but it pollutes `git status`. Run `git config --global core.autocrlf input` once to disable conversion on commit.
- **Long paths**: Some Go module paths exceed Windows' 260-char limit. Enable long paths via PowerShell as Admin: `New-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1 -PropertyType DWORD -Force`.
- **`gophish.db` locked**: SQLite holds an exclusive lock; if you `Ctrl+C` while a campaign is in flight, the DB may need WAL recovery on next start (handled automatically).
