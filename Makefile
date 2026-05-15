public:
	git checkout v4
	git pull
	git tag -a v4.1.8 -m "v4.1.8\r\n- Fixed message paser\r\n- Fixed not call dispose namespace when disconnected"
	git push origin v4.1.8
	env GOPROXY=proxy.golang.org
	go list -m github.com/doquangtan/socketio/v4@v4.1.8