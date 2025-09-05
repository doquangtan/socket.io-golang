public:
	git checkout v4
	git pull
	git tag -a v4.1.1 -m "v4.1.1: Rename module path to github.com/doquangtan/socketio/v4"
	git push origin v4.1.1
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.1