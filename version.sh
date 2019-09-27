#!/bin/bash
set -euxo pipefail

user="${1}"
repo="${2}"
token="${3}"

gh="https://api.github.com/repos/${user}/${repo}"
c="curl -sfL -u ${user}:${token}"
currentversion=$(${c} ${gh}/releases | jq -r '.[0].name' | tr -d v)
version="v$(echo "${currentversion} + 0.01" | bc)"
grep "userd ${version}" main.go || (echo "Wrong version in main.go" && exit 1)
git tag ${version}
git push origin master
release=$(${c} -XPOST ${gh}/releases -d '{ "tag_name": "'${version}'", "target_commitish": "master", "name": "'${version}'", "body": "'${version}'", "draft": false, "prerelease": false }')
url=$(echo "${release}" | jq -r .upload_url | cut -f1 -d'{')
$c -XPOST ${url}?name=${repo} --data-binary @./userd -H "Content-Type: application/binary"
