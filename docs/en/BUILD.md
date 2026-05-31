# BUILD.md

# Build

Instructions for compiling the binary, packaging installers, and generating resources.

## Requirements

- **Go 1.23+**
- **PowerShell 5.1+**
- **Inno Setup 6** (To generate the `.exe` installer)
- **MinGW-w64** (For testing with `-race`)

## Compile Binary

```powershell
.\scripts\build.ps1
```
The script:
1. Checks if icons changed and updates the `.syso` automatically.
2. Compiles `AuraRPC.exe` stripping the symbol table to minimize weight (`-ldflags "-s -w"`).

## Generate Installer

```powershell
.\scripts\package.ps1
```
Calls the standard build and uses Inno Setup to create `dist\AuraRPC-<version>-setup.exe`.

## Update Resources (Icons)

If images in `icons/` are modified:
```powershell
go run .\tools\buildres
```
Regenerates `cmd\app\rsrc_windows_amd64.syso`, injecting icons in multiple resolutions and the native manifest. Go will automatically link it on the next build.

## Testing

```powershell
go vet ./...
go test -count=1 -race ./...
```
*(The `-race` flag requires CGo enabled).*