param([string]$Go = 'go')

$ErrorActionPreference = 'Stop'
$version = (Get-Content -LiteralPath internal/version/VERSION -Raw).Trim()

# go run with a module version ignores the toolchain directive in this repository.
$toolchain = (Select-String -LiteralPath go.mod -Pattern '^toolchain (go\S+)$').Matches[0].Groups[1].Value
$previousToolchain = $env:GOTOOLCHAIN
try {
    $env:GOTOOLCHAIN = $toolchain
    & $Go run github.com/tc-hib/go-winres@v0.3.3 make --in assets/winres.json --out internal/win32/icon --arch amd64,arm64 --file-version $version --product-version $version
    exit $LASTEXITCODE
} finally {
    $env:GOTOOLCHAIN = $previousToolchain
}
