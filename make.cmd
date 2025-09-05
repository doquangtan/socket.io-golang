echo off
set func=%1
set version=%2

IF %func%==public (
    echo Start %func%:
    git checkout v4

    git pull

    git tag -a %version% -m "v4.0.11: \n- Fixed issues with using socket..Emit() concurrently. \n- Added file server for net/http"

    git push origin %version%

    SET GOPROXY=proxy.golang.org

    go list -m github.com/doquangtan/socket.io/v4@%version%
    
    echo Done %func%
)

@REM make public v4.0.11