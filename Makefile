test:
	for i in test/Dockerfile.*; do docker build -f $${i} -t $${i#test/Dockerfile.} . || exit 1; done
	for i in test/Dockerfile.*; do docker run -it -v $${GOPATH}/bin:/root/bin -v $${PWD}:/tmp/userd $${i#test/Dockerfile.} /tmp/userd/test.sh || exit 1; done


shell:
	docker run -it -v $${GOPATH}/bin:/root/bin golang:stretch bash


install:
	go install -ldflags "-s -w"


auth:
	test -n "${GITHUB_TOKEN}"


publish: auth install test
	./version.sh alexlance userd ${GITHUB_TOKEN}


.PHONY: test shell install auth publish
