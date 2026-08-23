[CmdletBinding()]
param(
    [Parameter()]
    [string]$Version = "v0.1.1",

    [Parameter()]
    [string]$OutputDirectory = "dist"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ($Version -notmatch '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$') {
    throw "Version must be a semantic version."
}

$normalizedVersion = $Version.TrimStart("v")
$repositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$outputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    [System.IO.Path]::GetFullPath($OutputDirectory)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $repositoryRoot $OutputDirectory))
}
$repositoryPrefix = $repositoryRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
if (-not $outputRoot.StartsWith($repositoryPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Output directory must remain inside the repository working tree."
}

if (Test-Path -LiteralPath $outputRoot) {
    Remove-Item -LiteralPath $outputRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $outputRoot | Out-Null

$targets = @(
    @{ OS = "linux"; Architecture = "amd64"; Extension = ".tar.gz" },
    @{ OS = "linux"; Architecture = "arm64"; Extension = ".tar.gz" },
    @{ OS = "windows"; Architecture = "amd64"; Extension = ".zip" },
    @{ OS = "darwin"; Architecture = "amd64"; Extension = ".tar.gz" },
    @{ OS = "darwin"; Architecture = "arm64"; Extension = ".tar.gz" }
)

$originalGOOS = $env:GOOS
$originalGOARCH = $env:GOARCH
$originalCGOEnabled = $env:CGO_ENABLED

try {
    foreach ($target in $targets) {
        $baseName = "git-casebook_{0}_{1}_{2}" -f $normalizedVersion, $target.OS, $target.Architecture
        $stage = Join-Path $outputRoot ("stage-" + $baseName)
        New-Item -ItemType Directory -Path $stage | Out-Null

        $binaryName = if ($target.OS -eq "windows") { "git-casebook.exe" } else { "git-casebook" }
        $binaryPath = Join-Path $stage $binaryName
        $env:GOOS = $target.OS
        $env:GOARCH = $target.Architecture
        $env:CGO_ENABLED = "0"

        & go build -trimpath -ldflags "-s -w -X github.com/itxcrusher/git-casebook/internal/version.Override=$normalizedVersion" -o $binaryPath ./cmd/git-casebook
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed for $($target.OS)/$($target.Architecture)."
        }

        Copy-Item -LiteralPath (Join-Path $repositoryRoot "LICENSE") -Destination $stage
        Copy-Item -LiteralPath (Join-Path $repositoryRoot "NOTICE") -Destination $stage
        Copy-Item -LiteralPath (Join-Path $repositoryRoot "README.md") -Destination $stage
        Copy-Item -LiteralPath (Join-Path $repositoryRoot "THIRD_PARTY_NOTICES.md") -Destination $stage

        $archivePath = Join-Path $outputRoot ($baseName + $target.Extension)
        if ($target.Extension -eq ".zip") {
            Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $archivePath
        } else {
            & tar -C $stage -czf $archivePath .
            if ($LASTEXITCODE -ne 0) {
                throw "Archive creation failed for $baseName."
            }
        }
        Remove-Item -LiteralPath $stage -Recurse -Force
    }
} finally {
    $env:GOOS = $originalGOOS
    $env:GOARCH = $originalGOARCH
    $env:CGO_ENABLED = $originalCGOEnabled
}

$checksumLines = Get-ChildItem -LiteralPath $outputRoot -File |
    Where-Object { $_.Name -ne "SHA256SUMS" } |
    Sort-Object Name |
    ForEach-Object {
        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
        "$hash  $($_.Name)"
    }
[System.IO.File]::WriteAllLines(
    (Join-Path $outputRoot "SHA256SUMS"),
    [string[]]$checksumLines,
    [System.Text.UTF8Encoding]::new($false)
)

Write-Output "Built $($targets.Count) GitCasebook $normalizedVersion artifacts in $outputRoot"
