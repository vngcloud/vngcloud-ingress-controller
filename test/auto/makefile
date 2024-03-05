test:
	# curl -H "Host: https-example.foo.com" https://180.93.181.81/webserver -k

	go clean -testcache && go test -timeout 99999s -v ./k8s.go ./test_vlb.go ./utils.go ./test1_test.go
	go clean -testcache && go test -timeout 99999s -v ./k8s.go ./test_vlb.go ./utils.go ./test2_test.go
	go clean -testcache && go test -timeout 99999s -v ./k8s.go ./test_vlb.go ./utils.go ./test3_test.go
	go clean -testcache && go test -timeout 99999s -v ./k8s.go ./test_vlb.go ./utils.go ./test4_test.go