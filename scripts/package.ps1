$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true

$version = (Get-Content -LiteralPath internal/version/VERSION -Raw).Trim()
if ($version -cnotmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
    throw "Expected major.minor.patch in internal/version/VERSION, found '$version'."
}

$changelog = Get-Content -LiteralPath CHANGELOG.md -Raw
$entry = [regex]::Match($changelog, '(?ms)^## \[(?<version>[0-9][^\]\r\n]*)\] - [0-9]{4}-[0-9]{2}-[0-9]{2}\r?\n(?<notes>.*?)(?=^## |^\[[^\]\r\n]+\]: |\z)')
$notes = $entry.Groups['notes'].Value.Trim()
if ($entry.Groups['version'].Value -cne $version -or !$notes) {
    throw "The latest dated section in CHANGELOG.md must have release notes for $version."
}

$buildDirectory = (New-Item -ItemType Directory -Path bin/package -Force).FullName
New-Item -ItemType Directory -Path dist -Force | Out-Null
$executable = Join-Path $buildDirectory 'MinecraftInputGuard.exe'
$versionOutput = Join-Path $buildDirectory 'version.txt'
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'

foreach ($architecture in 'amd64', 'arm64') {
    $env:GOARCH = $architecture
    go build -trimpath -ldflags '-H=windowsgui' -o $executable ./cmd/minecraft-input-guard

    $info = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($executable)
    if ($info.ProductVersion -cne $version -or $info.FileVersion -cne $version) {
        throw "The Windows version metadata for $architecture does not match $version."
    }

    if ($architecture -eq 'amd64' -and [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'X64') {
        $app = Start-Process -FilePath $executable -ArgumentList '-version' -RedirectStandardOutput $versionOutput -Wait -PassThru
        if ($app.ExitCode -ne 0 -or (Get-Content -LiteralPath $versionOutput -Raw).Trim() -cne $version) {
            throw "The executable did not report version $version."
        }
    }

    Compress-Archive -LiteralPath @($executable, 'config.example.json') -DestinationPath "dist/MinecraftInputGuard-v$version-windows-$architecture.zip" -Force
}

Set-Content -LiteralPath dist/release-notes.md -Value $notes -Encoding utf8
Write-Output "v$version"
