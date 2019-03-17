test:
	cd env && (for i in *; do docker build -f $$i -t $$i . || exit 1; done)
	cd env && (for i in *; do docker run -it -v $${GOPATH}/bin:/root/bin $${i} /root/bin/userd --repo https://github.com/alexlance/userd --realm test || exit 1; done)


shell:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch bash


install:
	go install -ldflags "-s -w"


publish: install
	test -n "${GITHUB_TOKEN}"
	./version.sh alexlance userd ${GITHUB_TOKEN}


.PHONY: test shell install publish
