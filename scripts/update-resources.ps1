param([string]$Go = 'go')

$ErrorActionPreference = 'Stop'
$version = (Get-Content -LiteralPath internal/version/VERSION -Raw).Trim()
& $Go run github.com/tc-hib/go-winres@v0.3.3 make --in assets/winres.json --out internal/win32/icon --arch amd64,arm64 --file-version $version --product-version $version
exit $LASTEXITCODE
