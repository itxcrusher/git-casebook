[CmdletBinding()]
param(
    [Parameter()]
    [string]$Version = "v0.1.1",

    [Parameter()]
    [string]$OutputDirectory = "dist",

    [Parameter()]
    [switch]$ListPublishAssets
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

$expectedArchives = @(
    "git-casebook_${normalizedVersion}_linux_amd64.tar.gz",
    "git-casebook_${normalizedVersion}_linux_arm64.tar.gz",
    "git-casebook_${normalizedVersion}_windows_amd64.zip",
    "git-casebook_${normalizedVersion}_darwin_amd64.tar.gz",
    "git-casebook_${normalizedVersion}_darwin_arm64.tar.gz"
)
$expectedPublishFiles = @($expectedArchives + "SHA256SUMS")

if (-not (Test-Path -LiteralPath $outputRoot -PathType Container)) {
    throw "Release artifact directory does not exist."
}

$entries = @(Get-ChildItem -LiteralPath $outputRoot -Force)
if ($entries.Count -ne $expectedPublishFiles.Count) {
    throw "Release directory must contain exactly six publishable files."
}

foreach ($entry in $entries) {
    if ($entry -isnot [System.IO.FileInfo] -or
        ($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
        throw "Release directory contains a non-regular entry: $($entry.Name)"
    }
}

$actualPublishFiles = @($entries | Select-Object -ExpandProperty Name | Sort-Object)
if (Compare-Object -ReferenceObject ($expectedPublishFiles | Sort-Object) -DifferenceObject $actualPublishFiles) {
    throw "Release directory does not match the expected publication asset set."
}

$checksumPath = Join-Path $outputRoot "SHA256SUMS"
$recorded = @{}
foreach ($line in Get-Content -LiteralPath $checksumPath) {
    if ($line -notmatch '^([0-9a-f]{64})  (.+)$') {
        throw "Invalid SHA256SUMS entry."
    }
    $recorded[$Matches[2]] = $Matches[1]
}

foreach ($name in $expectedArchives) {
    if (-not $recorded.ContainsKey($name)) {
        throw "Missing checksum for $name."
    }
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $outputRoot $name)).Hash.ToLowerInvariant()
    if ($actualHash -ne $recorded[$name]) {
        throw "SHA-256 mismatch for $name."
    }
}

if ($recorded.Count -ne $expectedArchives.Count) {
    throw "SHA256SUMS contains an unexpected entry."
}

if ($ListPublishAssets) {
    foreach ($name in $expectedPublishFiles) {
        Write-Output (Join-Path $outputRoot $name)
    }
} else {
    Write-Output "Verified $($expectedArchives.Count) release artifacts and SHA-256 checksums."
}
