package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
)

// distroCommands for different flavours of Linux
type distroCommands struct {
	addUser        func(string, string) []string
	delUser        func(string) []string
	changeShell    func(string, string) []string
	changePassword func(string, string) []string
	changeHomeDir  func(string, string) []string
	changeGroups   func(string, string) []string
	changeComment  func(string, string) []string
}

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

func GetOSCommands(flavour string) distroCommands {
	switch strings.ToLower(flavour) {
	case "centos:7":
		return distroCommands{
			addUser: func(username string, home string) []string {
				return []string{"adduser", "-m", "--home-dir", home, username}
			},
			delUser: func(username string) []string {
				return []string{"userdel", "--remove", "-f", username}
			},
			changeShell: func(username string, shell string) []string {
				return []string{"usermod", "--shell", shell, username}
			},
			changePassword: func(username string, password string) []string {
				return []string{"usermod", "--password", password, username}
			},
			changeHomeDir: func(username string, home string) []string {
				return []string{"usermod", "--move-home", "--home", home, username}
			},
			changeGroups: func(username string, groups string) []string {
				return []string{"usermod", "--groups", groups, username}
			},
			changeComment: func(username string, comment string) []string {
				return []string{"usermod", "--comment", comment, username}
			},
		}
	case "debian", "debian:8", "debian:9", "ubuntu:16.04", "ubuntu:18.04", "ubuntu:18.10", "ubuntu:19.04":
		return distroCommands{
			addUser: func(username string, home string) []string {
				return []string{"adduser", "--home", home, "--disabled-password", username}
			},
			delUser: func(username string) []string {
				return []string{"deluser", "--remove-home", username}
			},
			changeShell: func(username string, shell string) []string {
				return []string{"usermod", "--shell", shell, username}
			},
			changePassword: func(username string, password string) []string {
				return []string{"usermod", "--password", password, username}
			},
			changeHomeDir: func(username string, home string) []string {
				return []string{"usermod", "--move-home", "--home", home, username}
			},
			changeGroups: func(username string, groups string) []string {
				return []string{"usermod", "--groups", groups, username}
			},
			changeComment: func(username string, comment string) []string {
				return []string{"usermod", "--comment", comment, username}
			},
		}
	default:
		log.Fatalf("No config for operating system: %s", flavour)
	}
	return distroCommands{}
}
