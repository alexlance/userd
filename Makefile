.PHONY: test
test:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch /root/bin/userd --repo https://github.com/alexlance/userd --realm test


.PHONY: shell
shell:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch bash
