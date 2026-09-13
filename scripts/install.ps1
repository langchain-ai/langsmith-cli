#!/usr/bin/env pwsh
#Requires -Version 5.1

<#
.SYNOPSIS
    Installs the langsmith CLI on Windows.

.DESCRIPTION
    Downloads a langsmith release archive from GitHub, verifies its SHA256 hash
    against the release checksums.txt, and installs the binary into InstallDir.

    Layout: pure helpers first (functional core), then everything that touches
    the network or filesystem (imperative shell), then Invoke-Install.
    scripts/install.sh mirrors this structure function for function; keep the
    two in step.

.PARAMETER InstallDir
    Directory to install into. Defaults to
    $env:LOCALAPPDATA\Programs\langsmith\bin.

.PARAMETER Version
    Release tag to install, for example v0.4.1. Defaults to the latest release.

.EXAMPLE
    irm https://cli.langsmith.com/install.ps1 | iex

    Installs the latest release into the default directory.

.EXAMPLE
    .\install.ps1 -Version v0.4.1 -InstallDir C:\tools\langsmith

    Installs a specific release into a specific directory.

.EXAMPLE
    & ([scriptblock]::Create((irm https://cli.langsmith.com/install.ps1))) -Version v0.4.1

    Passes parameters while running straight from the web, which
    'irm ... | iex' cannot do.

.NOTES
    Set the GITHUB_TOKEN environment variable to authenticate the release
    lookup and avoid anonymous GitHub API rate limits. An authenticated GitHub
    CLI ('gh auth login') is used as a fallback when GITHUB_TOKEN is unset.

.LINK
    https://github.com/langchain-ai/langsmith-cli
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "",
    [string]$Version = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
# Invoke-WebRequest redraws a progress bar per chunk on Windows PowerShell, which
# costs more than the download itself.
$ProgressPreference = 'SilentlyContinue'
if ($PSVersionTable.PSVersion.Major -lt 6) {
    # Windows PowerShell may still default to TLS 1.0, which api.github.com rejects.
    [Net.ServicePointManager]::SecurityProtocol =
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
}

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

$Repo = 'langchain-ai/langsmith-cli'
$CliName = 'langsmith'
$BinaryName = 'langsmith.exe'
$ArchiveExt = 'zip'
$ChecksumsName = 'checksums.txt'
$ReleaseApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$DownloadBaseUrl = "https://github.com/$Repo/releases/download"
$UserInstallSubdir = 'Programs\langsmith\bin'
$FetchAttempts = 3

# ---------------------------------------------------------------------------
# Functional core
#
# Pure helpers: every input arrives as a parameter, every result is returned,
# and nothing here reads the environment or touches disk. The Resolve-* helpers
# return $null for unsupported values, leaving the error message to the caller.
# ---------------------------------------------------------------------------

function Resolve-OSName {
    param([string]$Platform)

    switch ($Platform.ToLowerInvariant()) {
        'windows' { 'windows' }
        default { $null }
    }
}

function Resolve-ArchName {
    param([string]$Machine)

    switch -Regex ($Machine) {
        '^(x64|amd64|x86_64)$' { 'amd64' }
        '^(arm64|aarch64)$' { 'arm64' }
        default { $null }
    }
}

function Resolve-ArchiveName {
    param([string]$OSName, [string]$ArchName)

    "${CliName}_${OSName}_${ArchName}.${ArchiveExt}"
}

function Resolve-InstallDir {
    # An explicit override wins, then the per-user location under LocalAppData.
    param([string]$Override, [string]$LocalAppData, [string]$UserHome)

    if ($Override) {
        return $Override
    }

    $root = if ($LocalAppData) { $LocalAppData } else { Join-Path $UserHome 'AppData\Local' }
    return (Join-Path $root $UserInstallSubdir)
}

function Resolve-TagName {
    param([string]$ReleaseJson)

    if (-not $ReleaseJson) {
        return $null
    }

    try {
        $release = $ReleaseJson | ConvertFrom-Json
    } catch {
        return $null
    }

    if ($release -and $release.PSObject.Properties.Name -contains 'tag_name') {
        return $release.tag_name
    }

    return $null
}

function Resolve-ExpectedChecksum {
    # Compare the whole file-name field. Do not widen to a prefix match: that
    # would accept the hash of a different, longer asset name.
    param([string]$ChecksumsText, [string]$ArchiveName)

    foreach ($line in ($ChecksumsText -split "`r?`n")) {
        $fields = $line.Trim() -split '\s+'
        # sha256sum marks binary mode with a leading * on the file name.
        if ($fields.Count -ge 2 -and $fields[1].TrimStart('*') -eq $ArchiveName) {
            return $fields[0]
        }
    }

    return $null
}

function Format-ComparablePath {
    param([string]$Path)

    if (-not $Path) {
        return $null
    }

    try {
        return [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
    } catch {
        return $null
    }
}

function Test-PathContainsDir {
    param([string]$Directory, [string]$PathValue)

    $candidate = Format-ComparablePath -Path $Directory
    if (-not $candidate) {
        return $false
    }

    foreach ($entry in ($PathValue -split ';')) {
        $normalized = Format-ComparablePath -Path $entry
        if ($normalized -and $normalized -ieq $candidate) {
            return $true
        }
    }

    return $false
}

function Format-PathInstructions {
    param([string]$Directory)

    @(
        ''
        "Add $Directory to your PATH."
        'For the current PowerShell session:'
        "  `$env:PATH = `"$Directory;`$env:PATH`""
        ''
        'To persist it for your user account:'
        "  [Environment]::SetEnvironmentVariable('Path', `"$Directory;`" + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')"
    )
}

# ---------------------------------------------------------------------------
# Imperative shell
#
# Everything below reads the environment, talks to the network, or writes to
# disk. Write-Host, Write-Warning, and throw stand in for install.sh's log,
# warn, and die.
# ---------------------------------------------------------------------------

function Get-PlatformName {
    # $IsWindows exists only on PowerShell 6+; Windows PowerShell 5.1 is Windows-only.
    if ($PSVersionTable.PSVersion.Major -lt 6) { return 'windows' }
    if ($IsWindows) { return 'windows' }
    if ($IsMacOS) { return 'darwin' }
    if ($IsLinux) { return 'linux' }
    return 'unknown'
}

function Get-MachineName {
    try {
        return [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        # RuntimeInformation needs .NET Framework 4.7.1+ on Windows PowerShell.
        return $env:PROCESSOR_ARCHITECTURE
    }
}

function Invoke-GitHubRequest {
    param([string]$Uri, [string]$Token)

    $headers = @{ 'Accept' = 'application/vnd.github+json' }
    if ($Token) {
        $headers['Authorization'] = "Bearer $Token"
    }

    return (Invoke-WebRequest -Uri $Uri -Headers $headers -UseBasicParsing).Content
}

function Invoke-ReleaseApi {
    # GITHUB_TOKEN, then an authenticated gh, then anonymous.
    if ($env:GITHUB_TOKEN) {
        return Invoke-GitHubRequest -Uri $ReleaseApiUrl -Token $env:GITHUB_TOKEN
    }

    if (Get-Command gh -ErrorAction SilentlyContinue) {
        try {
            $json = & gh api -H 'Accept: application/vnd.github+json' "repos/$Repo/releases/latest" 2>$null
            if ($LASTEXITCODE -eq 0 -and $json) {
                return ($json -join "`n")
            }
        } catch {
            # fall through to an anonymous request
        }
    }

    return Invoke-GitHubRequest -Uri $ReleaseApiUrl
}

function Get-LatestVersion {
    # Returns the latest release tag, or $null once the retries are spent.
    for ($attempt = 1; $attempt -le $FetchAttempts; $attempt++) {
        $tag = $null
        try {
            $tag = Resolve-TagName -ReleaseJson (Invoke-ReleaseApi)
        } catch {
            $tag = $null
        }

        if ($tag) {
            return $tag
        }

        if ($attempt -lt $FetchAttempts) {
            Start-Sleep -Seconds $attempt
        }
    }

    return $null
}

function Save-RemoteFile {
    param([string]$Uri, [string]$Path)

    Invoke-WebRequest -Uri $Uri -OutFile $Path -UseBasicParsing
}

function Get-FileSha256 {
    param([string]$Path)

    return (Get-FileHash -Algorithm SHA256 -Path $Path).Hash.ToLowerInvariant()
}

function Confirm-Checksum {
    param([string]$ArchivePath, [string]$ArchiveName, [string]$ChecksumsPath)

    $expected = Resolve-ExpectedChecksum `
        -ChecksumsText (Get-Content -Raw -Path $ChecksumsPath) `
        -ArchiveName $ArchiveName

    if (-not $expected) {
        Write-Warning "$ArchiveName is not listed in $ChecksumsName; skipping verification"
        return
    }

    $actual = Get-FileSha256 -Path $ArchivePath
    if ($actual -ne $expected.ToLowerInvariant()) {
        throw "Checksum verification failed for $ArchiveName`n  Expected: $expected`n  Actual:   $actual"
    }
}

function Install-ReleaseBinary {
    param([string]$SourcePath, [string]$Directory)

    New-Item -ItemType Directory -Path $Directory -Force | Out-Null
    Move-Item -Path $SourcePath -Destination (Join-Path $Directory $BinaryName) -Force
}

function Invoke-Install {
    param([string]$InstallDirOverride, [string]$VersionOverride)

    $platform = Get-PlatformName
    $machine = Get-MachineName

    $osName = Resolve-OSName -Platform $platform
    if (-not $osName) {
        throw "Unsupported OS: $platform. Use scripts/install.sh on Linux and macOS."
    }

    $archName = Resolve-ArchName -Machine $machine
    if (-not $archName) {
        throw "Unsupported architecture: $machine"
    }

    $installDir = Resolve-InstallDir `
        -Override $InstallDirOverride `
        -LocalAppData $env:LOCALAPPDATA `
        -UserHome $HOME

    $releaseTag = $VersionOverride
    if (-not $releaseTag) {
        $releaseTag = Get-LatestVersion
        if (-not $releaseTag) {
            throw @(
                'Failed to determine latest version (GitHub API unreachable or rate-limited).'
                "Set GITHUB_TOKEN, run 'gh auth login', or pass -Version to install a specific release."
            ) -join "`n"
        }
    }

    $archiveName = Resolve-ArchiveName -OSName $osName -ArchName $archName
    $archiveUrl = "$DownloadBaseUrl/$releaseTag/$archiveName"
    $checksumsUrl = "$DownloadBaseUrl/$releaseTag/$ChecksumsName"

    Write-Host "Installing $CliName $releaseTag ($osName/$archName)..."

    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("langsmith-install-" + [System.Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tempDir | Out-Null

    try {
        $archivePath = Join-Path $tempDir $archiveName
        $checksumsPath = Join-Path $tempDir $ChecksumsName
        $extractDir = Join-Path $tempDir 'extract'

        Save-RemoteFile -Uri $archiveUrl -Path $archivePath
        Save-RemoteFile -Uri $checksumsUrl -Path $checksumsPath
        Confirm-Checksum -ArchivePath $archivePath -ArchiveName $archiveName -ChecksumsPath $checksumsPath
        Expand-Archive -Path $archivePath -DestinationPath $extractDir -Force

        $sourceBinary = Join-Path $extractDir $BinaryName
        if (-not (Test-Path $sourceBinary)) {
            throw "Archive did not contain $BinaryName"
        }

        Install-ReleaseBinary -SourcePath $sourceBinary -Directory $installDir
        Write-Host "Installed $CliName to $(Join-Path $installDir $BinaryName)"

        if (-not (Test-PathContainsDir -Directory $installDir -PathValue $env:PATH)) {
            Format-PathInstructions -Directory $installDir | ForEach-Object { Write-Host $_ }
        }

        Write-Host ''
        Write-Host "Run: $CliName --version"
    } finally {
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
    }
}

Invoke-Install -InstallDirOverride $InstallDir -VersionOverride $Version
