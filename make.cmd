echo off
set func=%1
set version=%2

IF %func%==public (
    echo Start %func%:
    git checkout v4

    git pull

    git tag -a %version% -m "Releasing version v4.0.8"

    git push origin %version%

    SET GOPROXY=proxy.golang.org

    go list -m github.com/doquangtan/socket.io/v4@%version%
    
    echo Done %func%
)

@REM make public v4.0.8