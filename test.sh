#!/bin/bash

set -euxo pipefail


function remove_users() {
  echo "Removing users ######################################"
  local c="/tmp/userd/userd --repo ./ --realm deleteall"
  $c
  test 0 == $(ls -1 /home/ | wc -l)
  test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
}

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

echo "0 ######################################"
cat /etc/os-release

# get git setup out of the way
git config --global user.email "you@example.com"
git config --global user.name "Your Name"
git clone https://github.com/alexlance/userd ./data
cd data


echo "1 ######################################"
c="/tmp/userd/userd --repo https://github.com/alexlance/userd --realm test"
$c
check_users "alla andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "2 ######################################"
c="/tmp/userd/userd --repo ./ --realm test"
$c
check_users "alla andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "3 ######################################"
c="/tmp/userd/userd --repo ./ --realm dev"
$c
check_users "alla andy steve"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "4 ######################################"
c="/tmp/userd/userd --repo ./ --realm doesntexist"
$c
check_users "alla andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "5 ######################################"
change_value test.json '{ realms : []}'
c="/tmp/userd/userd --repo ./ --realm dev"
$c
check_users "andy steve"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "6 ######################################"
change_value test.json '{ realms : ["dev"]}'
c="/tmp/userd/userd --repo ./ --realm dev"
$c
check_users "andy steve alla"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "7 ######################################"
c="/tmp/userd/userd --repo ./ --realm test"
$c
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "8 ######################################"
change_value test3.json '{ comment : "King of all Ops"}'
c="/tmp/userd/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating comment for andy"
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
grep andy /etc/passwd | grep "King of all Ops"


echo "9 ######################################"
change_value test3.json '{ groups : ["audio","cdrom", "doesntexist"]}'
c="/tmp/userd/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating user groups for andy"
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
groups andy | grep audio
groups andy | grep cdrom
groups andy | grep doesntexist && exit 1


echo "10######################################"
change_value test3.json '{ password : "this my password"}'
change_value test3.json '{ shell : "/bin/sh"}'
c="/tmp/userd/userd --repo ./ --realm test"
output=$($c 2>&1 | tee /dev/stderr)
grep "Updating password for andy" <<< $output
grep "Updating shell for andy" <<< $output
check_users "andy"
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
grep andy /etc/shadow | grep "this my password"
grep andy /etc/passwd | grep "/bin/sh"


echo "11######################################"
change_value test3.json '{ ssh_keys : ["this my key1", "this is the second key", "third key"]}'
c="/tmp/userd/userd --repo ./ --realm test"
$c 2>&1 | tee /dev/stderr | grep "Updating ssh keys for andy"
check_users "andy"
test "$(cat /home/andy/.ssh/authorized_keys | wc -l)" -eq 2 # no newline in file
cat /home/andy/.ssh/authorized_keys
grep "this my key1" /home/andy/.ssh/authorized_keys
grep "this is the second key" /home/andy/.ssh/authorized_keys
grep "third key" /home/andy/.ssh/authorized_keys
test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1


echo "12######################################"
change_value test3.json '{ realms : []}'
change_value test.json '{ groups : ["sudo:taiii"]}'
remove_users


echo "13######################################"
change_value test.json '{ realms : ["devil"]}'
change_value test.json '{ groups : ["audio:heaven"]}'
c="/tmp/userd/userd --repo ./ --realm devil"
$c
check_users "alla"
groups alla | grep audio && exit 1

remove_users


echo "14######################################"
change_value test.json '{ groups : ["audio:devil"]}'
c="/tmp/userd/userd --repo ./ --realm devil"
$c
check_users "alla"
groups alla | grep audio

remove_users


echo "15######################################"
change_value test.json '{ groups : ["audio:devi*"]}'
c="/tmp/userd/userd --repo ./ --realm devil"
$c
check_users "alla"
groups alla | grep audio

remove_users


echo "16######################################"
change_value test.json '{ groups : ["nonexistentgroup:devi*"]}'
c="/tmp/userd/userd --repo ./ --realm devil"
$c
check_users "alla"
groups alla | grep nonexistentgroup && exit 1

remove_users


echo "17######################################"
groupadd nonexistentgroup
c="/tmp/userd/userd --repo ./ --realm devil"
$c
check_users "alla"
groups alla | grep nonexistentgroup # should exist now


echo "DONE"
#exit 1
