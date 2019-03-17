package main

import (
	"fmt"
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
	version := ""
	version_id := ""
	for _, line := range s {
		if bits := strings.Split(line, `=`); len(bits) > 0 {
			if bits[0] == "ID" {
				version = strings.Replace(bits[1], `"`, ``, -1)
			}
			if bits[0] == "VERSION_ID" {
				version_id = strings.Replace(bits[1], `"`, ``, -1)
			}
		}
	}

	if version != "" && version_id != "" {
		return fmt.Sprintf("%s:%s", version, version_id)
	} else if version != "" {
		return version
	} else {
		return ""
	}
}

func GetOSCommands(flavour string) Commands {
	switch strings.ToLower(flavour) {
	case "centos:7":
		return Commands{
			addUser:        `adduser -m --home-dir "%s" %s`,
			delUser:        `userdel --remove -f %s`,
			changeShell:    `usermod --shell %s %s`,
			changePassword: `usermod --password '%s' %s`,
			changeHomeDir:  `usermod --move-home --home %s %s`,
			changeGroups:   `usermod --groups %s %s`,
			changeComment:  `usermod --comment "%s" %s`,
		}
	case "debian", "debian:8", "debian:9", "ubuntu:16.04", "ubuntu:18.04", "ubuntu:18.10", "ubuntu:19.04":
		return Commands{
			addUser:        `adduser --home "%s" --disabled-password %s`,
			delUser:        `deluser --remove-home %s`,
			changeShell:    `usermod --shell %s %s`,
			changePassword: `usermod --password '%s' %s`,
			changeHomeDir:  `usermod --move-home --home %s %s`,
			changeGroups:   `usermod --groups %s %s`,
			changeComment:  `usermod --comment "%s" %s`,
		}
	default:
		log.Fatalf("No config for operating system: %s", flavour)
	}
	return Commands{}
}
