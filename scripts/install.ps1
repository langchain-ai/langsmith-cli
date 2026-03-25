#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

$Repo = "langchain-ai/langsmith-cli"
$Binary = "langsmith.exe"

function Get-Architecture {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Get-LatestVersion {
    $headers = @{
        "Accept" = "application/vnd.github+json"
    }
    if ($env:GITHUB_TOKEN) {
        $headers["Authorization"] = "Bearer $env:GITHUB_TOKEN"
    }

    $release = Invoke-RestMethod -Headers $headers -Uri "https://api.github.com/repos/$Repo/releases/latest"
    if (-not $release.tag_name) {
        throw "Failed to determine latest version"
    }

    return $release.tag_name
}

function Test-PathContainsDirectory {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Directory
    )

    $candidate = [System.IO.Path]::GetFullPath($Directory).TrimEnd('\')
    foreach ($entry in ($env:PATH -split ';')) {
        if (-not $entry) {
            continue
        }

        try {
            $normalized = [System.IO.Path]::GetFullPath($entry).TrimEnd('\')
            if ($normalized -ieq $candidate) {
                return $true
            }
        } catch {
            continue
        }
    }

    return $false
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $HOME "AppData\Local\Programs\langsmith\bin"
}

if (-not $Version) {
    $Version = Get-LatestVersion
}

$Arch = Get-Architecture
$Archive = "langsmith_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"
$ArchiveUrl = "$BaseUrl/$Archive"
$ChecksumUrl = "$BaseUrl/checksums.txt"

Write-Host "Installing langsmith $Version ($Arch)..."

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("langsmith-install-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
    $ArchivePath = Join-Path $TempDir $Archive
    $ChecksumPath = Join-Path $TempDir "checksums.txt"
    $ExtractDir = Join-Path $TempDir "extract"

    Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath
    Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath

    $Expected = Select-String -Path $ChecksumPath -Pattern ([regex]::Escape($Archive)) | ForEach-Object {
        ($_ -split '\s+')[0]
    } | Select-Object -First 1

    if ($Expected) {
        $Actual = (Get-FileHash -Algorithm SHA256 -Path $ArchivePath).Hash.ToLowerInvariant()
        if ($Actual -ne $Expected.ToLowerInvariant()) {
            throw "Checksum verification failed. Expected $Expected but got $Actual"
        }
    }

    Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDir -Force

    $SourceBinary = Join-Path $ExtractDir $Binary
    if (-not (Test-Path $SourceBinary)) {
        throw "Archive did not contain $Binary"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $TargetBinary = Join-Path $InstallDir $Binary
    Move-Item -Path $SourceBinary -Destination $TargetBinary -Force

    Write-Host "Installed langsmith to $TargetBinary"

    if (-not (Test-PathContainsDirectory -Directory $InstallDir)) {
        Write-Host ""
        Write-Host "Add $InstallDir to your PATH."
        Write-Host "For the current PowerShell session:"
        Write-Host "  `$env:PATH = `"$InstallDir;`$env:PATH`""
        Write-Host ""
        Write-Host "To persist it for your user account:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"$InstallDir;`" + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')"
    }

    Write-Host ""
    Write-Host "Run: langsmith --version"
} finally {
    if (Test-Path $TempDir) {
        Remove-Item -Path $TempDir -Recurse -Force
    }
}
