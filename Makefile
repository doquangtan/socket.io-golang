public:
	git checkout v4
	git pull
	git tag -a v4.1.9 -m "v4.1.9\r\n- Fixed deadlock at io.Of.To(room)\r\n- Fixed room chaining and deduplication"
	git push origin v4.1.9
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.9