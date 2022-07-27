$os = ""
$arch = "amd64"
$name = $MyInvocation.MyCommand.Name
if ($args.Count -lt 1) {
    Write-Output  "Usage: $name version [ARCH [OS]]"
    exit
}
if ($args.Count -gt 1) {
    $arch = $args[1]
}
if ($args.Count -gt 2) {
    $os = $args[2]
}
$BUILD_VERSION = $args[0]
$BUILD_NAME = "Tiger"
$BUILD_TIME = $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
$COMMIT_SHA1 = $(git rev-parse HEAD)

function DoBuild {
    param (
        $goos,
        $goarch
    )
    Write-Output "GOOS=$goos GOARCH=$goarch BUILD_VERSION=$BUILD_VERSION BUILD_NAME=$BUILD_NAME BUILD_TIME=$BUILD_TIME COMMIT_SHA1=$COMMIT_SHA1"
    if ($goos -ne "android") {
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $ldflags = @"
        -s -w
        -X 'main.BuildVersion=$BUILD_VERSION'
        -X 'main.BuildTime=$BUILD_TIME'
        -X 'main.CommitID=$COMMIT_SHA1'
        -X 'main.BuildName=$BUILD_NAME'
"@
        go build -o ./bin/ -ldflags $ldflags
        go build -o ./bin/ -ldflags $ldflags   ./batch/batch.go
    }
    else {
        gomobile bind -target=android -o ./bin/bnrtc2.aar ./bnrtc2_ccc
    }
    Write-Output "===============================================  $GOOS $GOARCH build ok"
}

$oldArch = $env:GOARCH
$oldOs = $env:GOOS

if ($os -ne "") {
    DoBuild $os $arch
}
else {
    $needles = "windows", "linux", "android"
    for ($i = 0; $i -lt $needles.Count; $i++) {
        $goos = $needles[$i]
        Write-Progress -Activity "Building" -Status "> $goos"    -PercentComplete (($i / $needles.Count) * 100)
        DoBuild $goos $arch
    }
}

$env:GOARCH = $oldArch
$env:GOOS = $oldOs