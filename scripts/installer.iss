; AuraRPC — Inno Setup installer script.
;
; Uso:
;   1. Compila el binario: ./scripts/build.ps1  (genera AuraRPC.exe en la raíz).
;   2. Abre este archivo con Inno Setup Compiler (ISCC.exe).
;   3. El instalador se deja en dist/.
;
; Notas:
;   - PrivilegesRequired=lowest: instala por usuario, sin elevación.
;   - AppId fijo para que el upgrade reemplace en sitio.
;   - El binario incluye su propio system tray; no añadimos shortcut en
;     Inicio porque el usuario activa AutoStart desde Settings.

#define MyAppName       "AuraRPC"
; Version can be overridden from the command line: ISCC /DMyAppVersion=1.0.0
#ifndef MyAppVersion
  #define MyAppVersion  "0.1.0"
#endif
#define MyAppPublisher  "AuraRPC"
#define MyAppExeName    "AuraRPC.exe"
#define MyAppIcon       "..\icons\AuraRPC.ico"

[Setup]
AppId={{B5E7F2B2-1F33-4D6E-9C7A-4F1C3E2A9D11}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputDir=..\dist
OutputBaseFilename=AuraRPC-{#MyAppVersion}-setup
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
DisableWelcomePage=no
SetupIconFile={#MyAppIcon}
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[Files]
Source: "..\AuraRPC.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"
Name: "{userdesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; Borra el directorio de datos del usuario solo si está vacío. Si el
; usuario quiere conservar sus presets entre instalaciones, no toques
; %APPDATA%\AuraRPC — la limpieza la hace el usuario manualmente.
Type: dirifempty; Name: "{userappdata}\{#MyAppName}"
