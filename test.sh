#!/bin/bash

set -euxo pipefail


function check_users() {
  local num=0
  for i in $1; do
    test -d /home/${i}
    num=$((num+1))
  done
  test ${num} == $(ls -1 /home/ | wc -l)
}

function change_value() {
  pushd test
  cat ${1} | jq ". + ${2}" > t
  mv t ${1}
  git add ${1}
  git commit -m "changed ${1} to ${2}"
  popd
}

echo "######################################"
cat /etc/os-release

# get git setup out of the way
git config --global user.email "you@example.com"
git config --global user.name "Your Name"
git clone https://github.com/alexlance/userd ./data
cd data


c="/root/bin/userd --repo https://github.com/alexlance/userd --realm test"
$c
check_users "alla andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


c="/root/bin/userd --repo ./ --realm test"
$c
check_users "alla andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


c="/root/bin/userd --repo ./ --realm dev"
$c
check_users "alla andy steve"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


change_value test.json '{ realms : []}'
c="/root/bin/userd --repo ./ --realm dev"
$c
check_users "andy steve"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1

change_value test.json '{ realms : ["dev"]}'
c="/root/bin/userd --repo ./ --realm dev"
$c
check_users "andy steve alla"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


c="/root/bin/userd --repo ./ --realm test"
$c
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


change_value test3.json '{ comment : "King of all Ops"}'
c="/root/bin/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating comment for andy"
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
grep andy /etc/passwd | grep "King of all Ops"


change_value test3.json '{ groups : ["audio","cdrom", "doesntexist"]}'
c="/root/bin/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating user groups for andy"
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
groups andy | grep audio
groups andy | grep cdrom
groups andy | grep doesntexist && exit 1


change_value test3.json '{ password : "this my password"}'
change_value test3.json '{ shell : "/bin/sh"}'
c="/root/bin/userd --repo ./ --realm test"
output=$($c 2>&1 | tee /dev/stderr)
grep "Updating password for andy" <<< $output
grep "Updating shell for andy" <<< $output
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
grep andy /etc/shadow | grep "this my password"
grep andy /etc/passwd | grep "/bin/sh"


change_value test3.json '{ ssh_keys : ["this my key1", "this is the second key", "third key"]}'
c="/root/bin/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating ssh keys for andy"
check_users "andy"
test "$(cat /home/andy/.ssh/authorized_keys | wc -l)" -eq 2 # no newline in file
cat /home/andy/.ssh/authorized_keys
grep "this my key1" /home/andy/.ssh/authorized_keys
grep "this is the second key" /home/andy/.ssh/authorized_keys
grep "third key" /home/andy/.ssh/authorized_keys
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
