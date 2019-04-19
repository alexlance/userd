#!/bin/bash

set -euxo pipefail


function run_test() {
  local name="${1}"
  local c="${2}"
  set +x
  echo "######################################################################### ${name}"
  set -x
  if ${c} 2>&1 | tee /dev/stderr | grep Error; then
    echo "Error detected"
    exit 1
  fi
  test "$($c 2>&1 | tee /dev/stderr | wc -l)" -eq 1
}

function remove_users() {
  set +x
  echo "Removing users..."
  set -x
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


run_test "1" "/tmp/userd/userd --repo https://github.com/alexlance/userd --realm test"
check_users "alla andy"

run_test "2" "/tmp/userd/userd --repo ./ --realm test"
check_users "alla andy"

run_test "3" "/tmp/userd/userd --repo ./ --realm dev"
check_users "alla andy steve"

run_test "4" "/tmp/userd/userd --repo ./ --realm doesntexist"
check_users "alla andy"

change_value test.json '{ realms : []}'
run_test "5" "/tmp/userd/userd --repo ./ --realm dev"
check_users "andy steve"

change_value test.json '{ realms : ["dev"]}'
run_test "6" "/tmp/userd/userd --repo ./ --realm dev"
check_users "andy steve alla"

run_test "7" "/tmp/userd/userd --repo ./ --realm test"
check_users "andy"

change_value test3.json '{ comment : "King of all Ops"}'
run_test "8" "/tmp/userd/userd --repo ./ --realm test" 2>&1 | grep "Updating comment for andy"
check_users "andy"
grep andy /etc/passwd | grep "King of all Ops"

change_value test3.json '{ groups : ["audio","cdrom", "doesntexist"]}'
run_test "9" "/tmp/userd/userd --repo ./ --realm test" 2>&1 | grep "Updating user groups for andy"
check_users "andy"
groups andy | grep audio
groups andy | grep cdrom
groups andy | grep doesntexist && exit 1

change_value test3.json '{ password : "this my password"}'
change_value test3.json '{ shell : "/bin/sh"}'
output=$(run_test "10" "/tmp/userd/userd --repo ./ --realm test" 2>&1)
grep "Updating password for andy" <<< $output
grep "Updating shell for andy" <<< $output
check_users "andy"
grep andy /etc/shadow | grep "this my password"
grep andy /etc/passwd | grep "/bin/sh"

change_value test3.json '{ ssh_keys : ["this my key1", "this is the second key", "third key"]}'
run_test "11" "/tmp/userd/userd --repo ./ --realm test" 2>&1 | grep "Updating ssh keys for andy"
check_users "andy"
test "$(cat /home/andy/.ssh/authorized_keys | wc -l)" -eq 2 # no newline in file
cat /home/andy/.ssh/authorized_keys
grep "this my key1" /home/andy/.ssh/authorized_keys
grep "this is the second key" /home/andy/.ssh/authorized_keys
grep "third key" /home/andy/.ssh/authorized_keys

change_value test3.json '{ realms : []}'
change_value test.json '{ groups : ["sudo:taiii"]}'
run_test "12" "echo hai mark"
remove_users

change_value test.json '{ realms : ["devil"]}'
change_value test.json '{ groups : ["audio:heaven"]}'
run_test "13" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep audio && exit 1
remove_users

change_value test.json '{ groups : ["audio:devil"]}'
run_test "14" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep audio
remove_users

change_value test.json '{ groups : ["audio:devi*"]}'
run_test "15" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep audio
remove_users


change_value test.json '{ groups : ["nonexistentgroup:devi*"]}'
run_test "16" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep nonexistentgroup && exit 1
remove_users


groupadd nonexistentgroup
run_test "17" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep nonexistentgroup # should exist now
remove_users

change_value test.json '{ groups : ["audio"]}'
run_test "18" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep audio


change_value test.json '{ groups : [""]}'
run_test "19" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
test "$(groups alla)" == "alla : alla"


change_value test.json '{ groups : ["audio"]}'
run_test "20" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
groups alla | grep audio


change_value test.json '{ groups : []}'
run_test "21" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
test "$(groups alla)" == "alla : alla"

change_value test.json '{ comment : "how do you:like them apples"}'
run_test "22" "/tmp/userd/userd --repo ./ --realm devil"
check_users "alla"
getent passwd alla


echo "DONE"
