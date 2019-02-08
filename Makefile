GITHUB_TOKEN ?= $(shell grep oauth_token ~/.config/hub | awk '{print $$2}')


.PHONY: test
test:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch /root/bin/userd --repo https://github.com/alexlance/userd --realm test


.PHONY: shell
shell:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch bash


.PHONY: install
install:
	go install -ldflags "-s -w"


.PHONY: publish
publish: install
	./version.sh alexlance userd $(GITHUB_TOKEN)
