test:
	go test -v ./...

lint:
	golangci-lint run

profile:
	go test -benchmem -run=^$$ -benchtime=5s  -bench ^BenchmarkEchoServ$$ -cpuprofile=cpu.out -memprofile=mem.out  -blockprofile=lock.out .

bench:
	go test -benchmem -run=^$$ -benchtime=5s  -bench ^BenchmarkEchoServ$$ .

analyze_cpu:
	go tool pprof -http=:8080 rpc-redis.test cpu.out

analyze_mem:
	go tool pprof -http=:8080 rpc-redis.test mem.out

analyze_block:
	go tool pprof -http=:8080 rpc-redis.test lock.out
