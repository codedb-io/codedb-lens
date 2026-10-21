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
AppSupportURL=https://github.com/codedb-io/codedb-lens/issues
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

[CustomMessages]
en.NoWebView2=The Microsoft Edge WebView2 Runtime was not found. The codeDB Lens window needs it to open (the command line works without it).%n%nOpen the download page now? Install the "Evergreen Bootstrapper", then run codeDB Lens.
ptbr.NoWebView2=O Microsoft Edge WebView2 Runtime não foi encontrado. A janela do codeDB Lens precisa dele para abrir (a linha de comando funciona sem ele).%n%nAbrir a página de download agora? Instale o "Evergreen Bootstrapper" e depois abra o codeDB Lens.

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

const
  WebView2Key = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';

function HasWebView2: Boolean;
var
  V: string;
begin
  Result := (RegQueryStringValue(HKLM32, WebView2Key, 'pv', V) or RegQueryStringValue(HKCU, WebView2Key, 'pv', V))
    and (V <> '') and (V <> '0.0.0.0');
end;

// Warn before installing, not after: the GUI fails to open without the runtime, and the installer
// never downloads anything itself.
function InitializeSetup: Boolean;
var
  Err: Integer;
begin
  Result := True;
  if not WizardSilent and not HasWebView2 then
    if MsgBox(CustomMessage('NoWebView2'), mbInformation, MB_YESNO) = IDYES then
      ShellExec('open', 'https://developer.microsoft.com/microsoft-edge/webview2/', '', '', SW_SHOWNORMAL, ewNoWait, Err);
end;

// Undo the "addtopath" task: leave no dangling entry pointing at a deleted folder.
procedure CurUninstallStepChanged(Step: TUninstallStep);
var
  Path, Dir: string;
  P: Integer;
begin
  if Step <> usUninstall then Exit;
  Dir := ExpandConstant('{app}');
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then Exit;
  Path := ';' + Path + ';';
  P := Pos(';' + Uppercase(Dir) + ';', Uppercase(Path));
  if P = 0 then Exit;
  Delete(Path, P, Length(Dir) + 1);
  if (Length(Path) > 0) and (Path[1] = ';') then Delete(Path, 1, 1);
  if (Length(Path) > 0) and (Path[Length(Path)] = ';') then Delete(Path, Length(Path), 1);
  RegWriteExpandStringValue(HKCU, 'Environment', 'Path', Path);
end;
