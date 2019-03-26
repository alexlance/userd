test: install
	for i in test/Dockerfile.*; do docker build -f $${i} -t $${i#test/Dockerfile.} . || exit 1; done
	for i in test/Dockerfile.*; do docker run -it -v $${PWD}:/tmp/userd $${i#test/Dockerfile.} /tmp/userd/test.sh || exit 1; done


install:
	go build -ldflags "-s -w"


auth:
	test -n "${GITHUB_TOKEN}"


publish: auth test
	./version.sh alexlance userd ${GITHUB_TOKEN}
	rm -f userd


.PHONY: test install auth publish
