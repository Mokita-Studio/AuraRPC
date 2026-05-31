# CONTRIBUTING.md

# Contributing

Technical guide for modifying the code.

## Environment

```powershell
git clone https://github.com/Mokita-Studio/AuraRPC.git
cd AuraRPC
go mod download
.\scripts\build.ps1
```

## Code Conventions

- **Formatting:** `gofmt` and `goimports` are mandatory.
- **Naming:** Exported identifiers use `PascalCase`, internals use `camelCase`. Acronyms in uppercase (`clientID`).
- **Comments:** Document exported symbols in English indicating *what* the block does. Architecture explanations belong in `.md` files.
- **Errors:** Return errors with context using `fmt.Errorf`. Reserve `panic` for unrecoverable startup failures.

## Architecture Rules

Respect the dependency direction:
- `internal/discord` must not import other `internal/` layers.
- `internal/ui` must not directly invoke `internal/discord` (it must use `core`).

## Adding Languages

1. Duplicate `internal/i18n/translations/en.json` naming it with its ISO code (e.g., `fr.json`).
2. Translate values respecting the keys.
3. The system will automatically detect it and add it to the UI options.

## Pull Requests

- Direct PRs to the `main` branch.
- Keep commits focused on a single task.
- Verify that tests and `go vet` pass cleanly.
- Do not add dependencies that require `cgo` or lack critical justification.