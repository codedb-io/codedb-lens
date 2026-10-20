; Per-user installer (no admin), so `codedb-lens update` can replace the binary without elevation.
; iscc /DAppVersion=0.1.0 /DSourceExe=..\..\dist\windows_windows_amd64_v1\codedb-lens.exe build\windows\installer.iss

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif
#ifndef SourceExe
  #define SourceExe "..\..\dist\codedb-lens.exe"
#endif

[Setup]
AppId={{6C1E2B4A-7A1D-4F57-9E0B-3C2D1B7A9F10}
AppName=codeDB Lens
AppVersion={#AppVersion}
AppPublisher=codeDB
AppPublisherURL=https://codedb.com.br/?utm_source=lens&utm_medium=installer&utm_campaign=publisher
AppSupportURL=https://github.com/codedb-monitorsql/codedb-lens/issues
DefaultDirName={localappdata}\Programs\codeDB Lens
DefaultGroupName=codeDB Lens
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\..\dist
OutputBaseFilename=codedb-lens_{#AppVersion}_windows_amd64_setup
SetupIconFile=..\..\assets\icon\icon.ico
UninstallDisplayIcon={app}\codedb-lens.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
ChangesEnvironment=yes

[Languages]
Name: "en"; MessagesFile: "compiler:Default.isl"
Name: "ptbr"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "addtopath"; Description: "Add codedb-lens to PATH (command line)"; Flags: unchecked

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\codeDB Lens"; Filename: "{app}\codedb-lens.exe"
Name: "{userdesktop}\codeDB Lens"; Filename: "{app}\codedb-lens.exe"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; \
  Tasks: addtopath; Check: NeedsAddPath(ExpandConstant('{app}'))

[Run]
Filename: "{app}\codedb-lens.exe"; Description: "{cm:LaunchProgram,codeDB Lens}"; Flags: nowait postinstall skipifsilent

[Code]
function NeedsAddPath(Dir: string): Boolean;
var
  Path: string;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then
    Path := '';
  Result := Pos(';' + Uppercase(Dir) + ';', ';' + Uppercase(Path) + ';') = 0;
end;
