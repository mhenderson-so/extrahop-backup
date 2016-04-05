param (
    [switch]$Debug,
    [switch]$Linux
    )

[string]$BinaryPath = ""
[string]$GOOS = ""
[string]$GOOSColour = ""

if ($Debug){
    if ($Linux){
        $BinaryPath = "extrahop-backup.debug"
    } else {
        $BinaryPath = "extrahop-backup.debug.exe"
    }
} else {
    if ($Linux){
        $BinaryPath = "extrahop-backup"
    } else {
        $BinaryPath = "extrahop-backup.exe"
    }
}

if ($Linux){
    $GOOS = "linux"
    $GOOSColour = "magenta"
} else {
    $GOOS = "windows"
    $GOOSColour = "DarkCyan"
}

$env:GOOS = $GOOS

if ($Debug){
    Write-Host "---- Building Debug (" -NoNewLine
} else {
    Write-Host "---- Building (" -NoNewLine
}

Write-Host $GOOS -f $GOOSColour -NoNewLine

Write-Host ") ----"
        
Write-Host "Removing existing $BinaryPath"
Remove-Item $BinaryPath -Force -ErrorAction SilentlyContinue

Write-Host "Building... $BinaryPath"

& go generate
if ($Debug){
    & godebug build -o $BinaryPath
} else {
    & go build -o $BinaryPath
}

if (-not (Test-Path $BinaryPath)) {
    Write-Host "Build Error" -f Red
}