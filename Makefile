public:
	git checkout v4
	git pull
	git tag -a v4.0.9 -m "Releasing version v4.0.9: - Fixed request origin not allowed by Upgrader.CheckOrigin"
	git push origin v4.0.9
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socket.io/v4@v4.0.9