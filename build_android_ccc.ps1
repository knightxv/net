$name = $MyInvocation.MyCommand.Name
if ($args.Count -lt 1) {
    Write-Output  "Usage: $name version"
    exit
}

$BUILD_VERSION = $args[0]
$BUILD_TIME = $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
$BUILD_NAME = "Tiger"
$COMMIT_SHA1 = $(git rev-parse HEAD)

Write-Output "BUILD_VERSION=$BUILD_VERSION  BUILD_TIME=$BUILD_TIME BUILD_NAME=$BUILD_NAME COMMIT_SHA1=$COMMIT_SHA1"
gomobile bind -target=android -o ./bin/bnrtc2.aar ./bnrtc2_ccc
Write-Output "===============================================  $GOOS $GOARCH build ok"