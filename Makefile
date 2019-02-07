.PHONY: test
test: install
	docker run -it -v $${PWD}:/root/bin golang:stretch /root/bin/userd --repo https://github.com/alexlance/userd --realm test


.PHONY: shell
shell:
	docker run -it -v $${PWD}:/root/bin golang:stretch bash

.PHONY: install
install:
	GOOS=linux go build ./...
