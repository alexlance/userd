package main

import (
	"io/ioutil"
	"log"
	"strings"
)

func GetOS() string {
	b, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		log.Fatal(err)
	}
	s := strings.Split(string(b), "\n")
	for _, line := range s {
		if bits := strings.Split(line, `=`); len(bits) > 0 {
			if bits[0] == "ID" {
				return strings.Replace(bits[1], `"`, ``, -1)
			}
		}
	}
	return ""
}

func GetOSCommands(flavour string) Commands {
	switch strings.ToLower(flavour) {
	case "debian":
		return Commands{
			addUser:        "adduser --disabled-password %s",
			delUser:        "deluser --remove-home %s",
			changeShell:    "usermod --shell %s %s",
			changePassword: "usermod --password '%s' %s",
			changeHomeDir:  "usermod --move-home --home %s %s",
			changeGroups:   "usermod --groups %s %s",
			changeComment:  "usermod --comment \"%s\" %s",
		}

	case "centos":
		return Commands{
			addUser:        "adduser --disabled-password %s",
			delUser:        "deluser --remove-home %s",
			changeShell:    "usermod --shell %s %s",
			changePassword: "usermod --password '%s' %s",
			changeHomeDir:  "usermod --move-home --home %s %s",
			changeGroups:   "usermod --groups %s %s",
			changeComment:  "usermod --comment \"%s\" %s",
		}
	default:
		log.Fatal("Unable to detect operating system")
	}
	return Commands{}
}
