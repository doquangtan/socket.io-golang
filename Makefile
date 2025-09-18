public:
	git checkout v4
	git pull
	git tag -a v4.1.6 -m "v4.1.6: - Fixed concurrent write not Lock Mutex when Ping to client."
	git push origin v4.1.6
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.6