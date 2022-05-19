package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
)

// distroCommands for different flavours of Linux
type distroCommands struct {
	addUser        func(string, string) ([]byte, error)
	delUser        func(string) ([]byte, error)
	changeShell    func(string, string) ([]byte, error)
	changePassword func(string, string) ([]byte, error)
	changeHomeDir  func(string, string) ([]byte, error)
	changeGroups   func(string, string) ([]byte, error)
	changeComment  func(string, string) ([]byte, error)
}

// get the short string version of the operating system eg debian:9
func getOS() string {
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

// return an operating-system specific user management command to run
func getOSCommands(flavour string) distroCommands {
	switch strings.ToLower(flavour) {
	case "centos:7", "centos:7.4", "centos:7.5", "centos:7.6":
		return distroCommands{
			addUser: func(username string, home string) ([]byte, error) {
				args := []string{"adduser", "-m", "--home-dir", home, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			delUser: func(username string) ([]byte, error) {
				args := []string{"userdel", "--remove", "-f", username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeShell: func(username string, shell string) ([]byte, error) {
				args := []string{"usermod", "--shell", shell, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changePassword: func(username string, password string) ([]byte, error) {
				args := []string{"usermod", "--password", password, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeHomeDir: func(username string, home string) ([]byte, error) {
				args := []string{"usermod", "--move-home", "--home", home, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeGroups: func(username string, groups string) ([]byte, error) {
				args := []string{"usermod", "--groups", groups, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeComment: func(username string, comment string) ([]byte, error) {
				args := []string{"usermod", "--comment", comment, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
		}
	case "debian", "debian:8", "debian:9", "debian:10", "debian:11", "ubuntu:16.04", "ubuntu:18.04", "ubuntu:18.10", "ubuntu:19.04":
		return distroCommands{
			addUser: func(username string, home string) ([]byte, error) {
				args := []string{"adduser", "--home", home, "--disabled-password", username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			delUser: func(username string) ([]byte, error) {
				args := []string{"deluser", "--remove-home", username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeShell: func(username string, shell string) ([]byte, error) {
				args := []string{"usermod", "--shell", shell, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changePassword: func(username string, password string) ([]byte, error) {
				args := []string{"usermod", "--password", password, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeHomeDir: func(username string, home string) ([]byte, error) {
				args := []string{"usermod", "--move-home", "--home", home, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeGroups: func(username string, groups string) ([]byte, error) {
				args := []string{"usermod", "--groups", groups, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
			changeComment: func(username string, comment string) ([]byte, error) {
				args := []string{"usermod", "--comment", comment, username}
				return exec.Command(args[0], args[1:]...).CombinedOutput()
			},
		}
	default:
		log.Fatalf("No config for operating system: %s", flavour)
	}
	return distroCommands{}
}
