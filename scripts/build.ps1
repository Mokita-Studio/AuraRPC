# Build de AuraRPC para Windows.
#
# Antes de tocar icons/ ejecuta también:
#   go run ./tools/buildres
# que regenera icons\AuraRPC.ico y cmd\app\rsrc_windows_amd64.syso. El .syso
# se enlaza al binario automáticamente y aporta el icono que ven el
# Explorador y la barra de tareas.

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# Regenera recursos sólo si el .syso es más antiguo que cualquiera de los PNG.
$syso = Join-Path $root "cmd\app\rsrc_windows_amd64.syso"
$pngs = Get-ChildItem (Join-Path $root "icons") -Filter "W*.png" -File
$needs = -not (Test-Path $syso)
if (-not $needs) {
    $sysoTime = (Get-Item $syso).LastWriteTime
    foreach ($p in $pngs) {
        if ($p.LastWriteTime -gt $sysoTime) { $needs = $true; break }
    }
}
if ($needs) {
    Write-Host "Regenerando recursos (icono .syso) ..."
    go run ./tools/buildres
}

go build -ldflags "-s -w -H windowsgui" -o AuraRPC.exe ./cmd/app
