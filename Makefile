public:
	git checkout v4
	git pull
	git tag -a v4.0.10 -m "Releasing version v4.0.10: - Change the name function authorization to authentication"
	git push origin v4.0.10
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socket.io/v4@v4.0.10