test:
	go test -v ./...

lint:
	golangci-lint run

profile:
	go test -benchmem -run=^$$ -bench ^BenchmarkEchoServ$$ -cpuprofile=cpu.out -memprofile=mem.out .

analyze_cpu:
	go tool pprof -http=:8080 rpc-redis.test cpu.out

analyze_mem:
	go tool pprof -http=:8080 rpc-redis.test mem.out
