public:
	git checkout v4
	git pull
	git tag -a v4.1.2 -m "v4.1.2: - Fixed error not found socket.io.min.js when get that file from package directory."
	git push origin v4.1.2
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.2