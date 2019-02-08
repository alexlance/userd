GITHUB_TOKEN ?= $(shell grep oauth_token ~/.config/hub | awk '{print $$2}')

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

.PHONY: publish
publish: install
	./version.sh alexlance userd $(GITHUB_TOKEN)
