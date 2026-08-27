$ErrorActionPreference = "Stop"

$repo = "tanaka-mambinge/takealot-plugin"
$releaseBase = $env:TAKEALOT_RELEASE_BASE_URL
if ([string]::IsNullOrWhiteSpace($releaseBase)) {
    $releaseBase = "https://github.com/$repo/releases/latest/download"
}

$architecture = $env:PROCESSOR_ARCHITECTURE.ToUpperInvariant()
switch ($architecture) {
    "AMD64" { $asset = "takealot_windows_amd64.exe" }
    "ARM64" { $asset = "takealot_windows_arm64.exe" }
    default { throw "Unsupported Takealot CLI architecture: $architecture." }
}

$cacheDir = $env:TAKEALOT_CLI_HOME
if ([string]::IsNullOrWhiteSpace($cacheDir)) {
    $cacheDir = Join-Path $HOME ".takealot\bin"
}
$null = New-Item -ItemType Directory -Force -Path $cacheDir
$null = & attrib +h $cacheDir 2>$null

$guid = [Guid]::NewGuid().ToString("N")
$binaryTemp = Join-Path $cacheDir ".takealot-cli-$guid.tmp"
$checksumsTemp = Join-Path $cacheDir ".takealot-checksums-$guid.tmp"
$target = Join-Path $cacheDir "takealot.exe"

try {
    Invoke-WebRequest -Uri "$releaseBase/$asset" -OutFile $binaryTemp
    Invoke-WebRequest -Uri "$releaseBase/checksums.txt" -OutFile $checksumsTemp

    $checksumLine = Get-Content -LiteralPath $checksumsTemp |
        Where-Object { $_ -match "(^|\s)$([regex]::Escape($asset))$" } |
        Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($checksumLine)) {
        throw "No checksum found for $asset."
    }

    $expected = ($checksumLine -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $binaryTemp).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "Checksum verification failed for $asset."
    }

    Move-Item -Force -LiteralPath $binaryTemp -Destination $target
    Write-Output $target
}
finally {
    Remove-Item -Force -ErrorAction SilentlyContinue -LiteralPath $binaryTemp, $checksumsTemp
}
