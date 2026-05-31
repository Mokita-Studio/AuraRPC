# Empaquetado: compila el binario y genera el instalador Inno Setup.
#
# Requisitos:
#   - Go 1.22+ en PATH (para go build).
#   - Inno Setup Compiler — busca ISCC.exe en la ruta estándar de
#     Program Files (x86) o en PATH.
#
# Salida: dist\AuraRPC-<ver>-setup.exe

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

# 1) Build del binario.
& "$PSScriptRoot\build.ps1"

# 2) Localizar ISCC.
$candidates = @(
    "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "ISCC.exe"
)
$iscc = $null
foreach ($c in $candidates) {
    if (Get-Command $c -ErrorAction SilentlyContinue) { $iscc = $c; break }
}
if (-not $iscc) {
    throw "Inno Setup not found. Install from https://jrsoftware.org/isinfo.php"
}

# 3) Compilar instalador.
New-Item -ItemType Directory -Force -Path "$root\dist" | Out-Null
& $iscc "$PSScriptRoot\installer.iss"

Write-Host "Installer written to dist\"
