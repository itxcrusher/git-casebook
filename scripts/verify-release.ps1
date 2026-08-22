[CmdletBinding()]
param(
    [Parameter()]
    [string]$Version = "v0.1.0",

    [Parameter()]
    [string]$OutputDirectory = "dist"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$normalizedVersion = $Version.TrimStart("v")
$repositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$outputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    [System.IO.Path]::GetFullPath($OutputDirectory)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $repositoryRoot $OutputDirectory))
}

$expected = @(
    "git-casebook_${normalizedVersion}_linux_amd64.tar.gz",
    "git-casebook_${normalizedVersion}_linux_arm64.tar.gz",
    "git-casebook_${normalizedVersion}_windows_amd64.zip",
    "git-casebook_${normalizedVersion}_darwin_amd64.tar.gz",
    "git-casebook_${normalizedVersion}_darwin_arm64.tar.gz"
) | Sort-Object

$actual = Get-ChildItem -LiteralPath $outputRoot -File |
    Where-Object { $_.Name -ne "SHA256SUMS" } |
    Select-Object -ExpandProperty Name |
    Sort-Object
if (Compare-Object -ReferenceObject $expected -DifferenceObject $actual) {
    throw "Release artifact set does not match the five supported targets."
}

$checksumPath = Join-Path $outputRoot "SHA256SUMS"
$recorded = @{}
foreach ($line in Get-Content -LiteralPath $checksumPath) {
    if ($line -notmatch '^([0-9a-f]{64})  (.+)$') {
        throw "Invalid SHA256SUMS entry."
    }
    $recorded[$Matches[2]] = $Matches[1]
}

foreach ($name in $expected) {
    if (-not $recorded.ContainsKey($name)) {
        throw "Missing checksum for $name."
    }
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $outputRoot $name)).Hash.ToLowerInvariant()
    if ($actualHash -ne $recorded[$name]) {
        throw "SHA-256 mismatch for $name."
    }
}

if ($recorded.Count -ne $expected.Count) {
    throw "SHA256SUMS contains an unexpected entry."
}

Write-Output "Verified $($expected.Count) release artifacts and SHA-256 checksums."
