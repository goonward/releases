[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$SourceRoot,
    [Parameter(Mandatory)][string]$OutputDir,
    [Parameter(Mandatory)][string]$Version,
    [Parameter(Mandatory)][string]$ToolsRef,
    [Parameter(Mandatory)][string]$GoplsRef
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ReleaseRoot = Split-Path -Parent $PSScriptRoot
$SourceRoot = [IO.Path]::GetFullPath($SourceRoot)
$OutputDir = [IO.Path]::GetFullPath($OutputDir)
$TempRoot = if ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { [IO.Path]::GetTempPath() }
$Work = Join-Path $TempRoot ("goplus-release-" + [guid]::NewGuid().ToString("N"))
$Stage = Join-Path $Work "payload"
$StageGo = Join-Path $Stage "go"
$StageBin = Join-Path $Stage "bin"
$StageLibexec = Join-Path $Stage "libexec"

function Invoke-Native {
    param(
        [Parameter(Mandatory)][string]$File,
        [Parameter(Mandatory)][string[]]$Arguments
    )
    & $File @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$File exited with status $LASTEXITCODE"
    }
}

New-Item -ItemType Directory -Force -Path $OutputDir, $StageGo, $StageBin, $StageLibexec | Out-Null
Set-Content -LiteralPath (Join-Path $Stage ".goplus-managed") -Value "Go+ $Version managed installation" -NoNewline

$OriginalGOROOT = $env:GOROOT
$OriginalGOTOOLCHAIN = $env:GOTOOLCHAIN
$OriginalGOWORK = $env:GOWORK

try {
    Write-Host "Building Go+"
    Push-Location (Join-Path $SourceRoot "src")
    try {
        Invoke-Native -File "cmd.exe" -Arguments @("/d", "/c", "make.bat")
    } finally {
        Pop-Location
    }

    foreach ($Directory in @("api", "bin", "doc", "lib", "misc", "src", "test")) {
        Copy-Item -Recurse -Force -LiteralPath (Join-Path $SourceRoot $Directory) -Destination $StageGo
    }
    $StagePkg = Join-Path $StageGo "pkg"
    New-Item -ItemType Directory -Force -Path $StagePkg | Out-Null
    Copy-Item -Recurse -Force -LiteralPath (Join-Path $SourceRoot "pkg\include") -Destination $StagePkg
    Copy-Item -Recurse -Force -LiteralPath (Join-Path $SourceRoot "pkg\tool") -Destination $StagePkg
    foreach ($File in @("CONTRIBUTING.md", "LICENSE", "PATENTS", "README.md", "SECURITY.md", "codereview.cfg", "go.env")) {
        $Path = Join-Path $SourceRoot $File
        if (Test-Path -LiteralPath $Path -PathType Leaf) {
            Copy-Item -Force -LiteralPath $Path -Destination (Join-Path $StageGo $File)
        }
    }
    Copy-Item -Force -LiteralPath (Join-Path $SourceRoot "VERSION.cache") -Destination (Join-Path $StageGo "VERSION")

    Write-Host "Building patched goimports and gopls"
    $ToolsDir = Join-Path $Work "tools"
    Invoke-Native -File "git.exe" -Arguments @("clone", "--quiet", "--filter=blob:none", "--no-checkout", "https://go.googlesource.com/tools", $ToolsDir)
    Invoke-Native -File "git.exe" -Arguments @("-C", $ToolsDir, "checkout", "--quiet", $ToolsRef)
    Invoke-Native -File "git.exe" -Arguments @("-C", $ToolsDir, "apply", (Join-Path $ReleaseRoot "patches\x-tools.patch"))

    $GoplsRepo = Join-Path $Work "gopls-repo"
    Invoke-Native -File "git.exe" -Arguments @("clone", "--quiet", "--filter=blob:none", "--no-checkout", "https://go.googlesource.com/tools", $GoplsRepo)
    Invoke-Native -File "git.exe" -Arguments @("-C", $GoplsRepo, "checkout", "--quiet", $GoplsRef)
    Invoke-Native -File "git.exe" -Arguments @("-C", $GoplsRepo, "apply", "--directory=gopls", (Join-Path $ReleaseRoot "patches\gopls.patch"))

    $PrivateGo = Join-Path $StageGo "bin\go.exe"
    $env:GOROOT = $StageGo
    $env:GOTOOLCHAIN = "local"
    $env:GOWORK = "off"
    Push-Location $ToolsDir
    try {
        Invoke-Native -File $PrivateGo -Arguments @("build", "-trimpath", "-o", (Join-Path $StageLibexec "goimports.exe"), "./cmd/goimports")
    } finally {
        Pop-Location
    }
    Push-Location (Join-Path $GoplsRepo "gopls")
    try {
        Invoke-Native -File $PrivateGo -Arguments @("mod", "edit", "-replace=golang.org/x/tools=$ToolsDir")
        Invoke-Native -File $PrivateGo -Arguments @("build", "-trimpath", "-o", (Join-Path $StageLibexec "gopls.exe"), ".")
    } finally {
        Pop-Location
    }

    Write-Host "Building Go+ launchers"
    $Launcher = Join-Path $StageBin "goplus-launcher.exe"
    Push-Location $ReleaseRoot
    try {
        Invoke-Native -File $PrivateGo -Arguments @("build", "-trimpath", "-o", $Launcher, "./cmd/launcher")
    } finally {
        Pop-Location
    }
    foreach ($Name in @("go+", "gofmt+", "gopls+", "goimports+")) {
        Copy-Item -Force -LiteralPath $Launcher -Destination (Join-Path $StageBin "$Name.exe")
    }
    Remove-Item -Force -LiteralPath $Launcher
    Invoke-Native -File (Join-Path $StageBin "go+.exe") -Arguments @("version")
    Invoke-Native -File (Join-Path $StageBin "gopls+.exe") -Arguments @("version")

    $Archive = Join-Path $OutputDir "goplus-windows-amd64.zip"
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::CreateFromDirectory($Stage, $Archive, [IO.Compression.CompressionLevel]::Optimal, $false)

    $Msi = Join-Path $OutputDir "goplus-windows-amd64.msi"
    Push-Location $ReleaseRoot
    try {
        Invoke-Native -File $PrivateGo -Arguments @(
            "run", "./cmd/msi",
            "-payload", $Stage,
            "-out", $Msi,
            "-work", (Join-Path $Work "msi"),
            "-version", $Version,
            "-arch", "amd64"
        )
    } finally {
        Pop-Location
    }
    Remove-Item -Force -LiteralPath ([IO.Path]::ChangeExtension($Msi, ".wixpdb")) -ErrorAction SilentlyContinue
} finally {
    if ($null -eq $OriginalGOROOT) { Remove-Item Env:GOROOT -ErrorAction SilentlyContinue } else { $env:GOROOT = $OriginalGOROOT }
    if ($null -eq $OriginalGOTOOLCHAIN) { Remove-Item Env:GOTOOLCHAIN -ErrorAction SilentlyContinue } else { $env:GOTOOLCHAIN = $OriginalGOTOOLCHAIN }
    if ($null -eq $OriginalGOWORK) { Remove-Item Env:GOWORK -ErrorAction SilentlyContinue } else { $env:GOWORK = $OriginalGOWORK }
    if (Test-Path -LiteralPath $Work) { Remove-Item -Recurse -Force -LiteralPath $Work }
}
