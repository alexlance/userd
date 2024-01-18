test:
	for i in test/Dockerfile.*; do docker build -f $${i} -t $${i#test/Dockerfile.} . || exit 1; done
	for i in test/Dockerfile.*; do docker run -it $${i#test/Dockerfile.} /tmp/userd/test.sh || exit 1; done


build:
	CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-s -w" -o userd .


auth:
	test -n "${GITHUB_TOKEN}"


publish: auth test build
	./version.sh alexlance userd ${GITHUB_TOKEN}
	rm -f userd


.PHONY: test build auth publish
