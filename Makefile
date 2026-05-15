public:
	git checkout v4
	git pull
	git tag -a v4.1.7 -m "v4.1.7\r\n- Added polling protocol\r\n- Added methods server.use\r\n- Added attribute socket.handshake"
	git push origin v4.1.7
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.7