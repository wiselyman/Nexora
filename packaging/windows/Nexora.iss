#ifndef AppVersion
  #error AppVersion must be defined
#endif
#ifndef TargetArch
  #error TargetArch must be defined
#endif
#ifndef BinaryPath
  #error BinaryPath must be defined
#endif
#ifndef OutputPath
  #error OutputPath must be defined
#endif

#define NexoraAppName "Nexora"

[Setup]
AppId={{9BCB6E5C-7BD2-4EF8-9BAF-513EE3A0DB10}
AppName={#NexoraAppName}
AppVersion={#AppVersion}
AppVerName={#NexoraAppName} {#AppVersion}
AppPublisher=Yunfei Wang
AppPublisherURL=https://github.com/wiselyman/Nexora
DefaultDirName={localappdata}\Programs\Nexora
DefaultGroupName=Nexora
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\Nexora.exe
OutputDir={#OutputPath}
OutputBaseFilename=Nexora-{#AppVersion}-windows-{#TargetArch}-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupLogging=yes
Uninstallable=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "chinesesimp"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "创建桌面快捷方式"; GroupDescription: "其他选项："; Flags: unchecked

[Files]
Source: "{#BinaryPath}"; DestDir: "{app}"; DestName: "Nexora.exe"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\Nexora"; Filename: "{app}\Nexora.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Nexora"; Filename: "{app}\Nexora.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\Nexora.exe"; Description: "启动 Nexora；启动后可访问 http://localhost:18080"; WorkingDir: "{app}"; Flags: postinstall nowait skipifsilent
