GOBUILD=env GOOS=linux go build -ldflags="-s -w" -o

build:
	$(GOBUILD) bin/slave slave/slave_server.go
	$(GOBUILD) bin/master master/master_server.go

deploy:
	sls deploy