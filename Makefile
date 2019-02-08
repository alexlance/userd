.PHONY: test
test: install
	docker run -it -v $${PWD}:/root golang:stretch /root/userd --repo https://github.com/alexlance/userd --realm test


.PHONY: shell
shell:
	docker run -it -v $${PWD}:/root golang:stretch bash

.PHONY: gotest
gotest:
	docker run -it -v $${PWD}:/root golang:stretch bash -c "cd /root && go test -v"

.PHONY: install
install:
	GOOS=linux go build ./...
