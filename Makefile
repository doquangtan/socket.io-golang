public:
	git checkout v4
	git pull
	git tag -a v4.0.11 -m "v4.0.11: \n- Fixed issues with using socket..Emit() concurrently. \n- Added file server for net/http"
	git push origin v4.0.11
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socket.io/v4@v4.0.11