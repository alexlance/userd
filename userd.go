package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

// structure consistent with chef users
type User struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
	Realms   []string `json:"realms"`
	SSHKeys  []string `json:"ssh_keys"`
}

// initial checks, all systems go?
func validate(realm string, repo string) {
	if realm == "" {
		log.Fatal("Error: Empty argument --realm")
	}
	if repo == "" {
		log.Fatal("Error: Empty argument --repo")
	}
	if os.Geteuid() != 0 {
		log.Fatalf("Error: Bad user id (%d), must run as root", os.Geteuid())
	}
	for _, cmd := range []string{"adduser", "deluser", "usermod", "git", "id", "getent"} {
		if _, err := exec.LookPath(cmd); err != nil {
			log.Fatalf("Error: Command not found: %s", cmd)
		}
	}
}

// go and grab a git repo full of json users
func pull_or_clone(repo string, dest string) {
	dir := path.Base(strings.Split(repo, " ")[0])
	var cmd *exec.Cmd
	if os.Chdir(path.Join(dest, dir)) == nil {
		log.Println("Running git pull")
		cmd = exec.Command("git", "pull")
	} else {
		log.Println("Running git clone")
		cmd = exec.Command("git", "clone", repo, path.Join(dest, dir))
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatal("Error: ", string(out), err)
	}
}

// gather all the users together who are meant to be in this instance's realm
func gather_json_users(repo string, dest string) map[string]User {
	dir := path.Base(strings.Split(repo, " ")[0])
	files, err := ioutil.ReadDir(path.Join(dest, dir))
	if err != nil {
		log.Fatal("Error: ", err)
	}
	users := make(map[string]User)
	just_usernames := []string{}
	re := regexp.MustCompile(`\s+`)
	for _, f := range files {
		name := f.Name()
		if f.IsDir() != true && len(name) > 5 && strings.ToLower(name[len(name)-5:]) == ".json" {
			if content, err := ioutil.ReadFile(name); err == nil {
				var u User
				compact := strings.TrimSpace(string(re.ReplaceAll(content, []byte(" "))))
				if err := json.Unmarshal(content, &u); err != nil {
					log.Printf("%s: Error: Parse or type error in JSON: %s", name, compact)
				} else if u.Username == "" {
					log.Printf("%s: Error: Missing 'username' in JSON: %s", name, compact)
				} else {
					users[u.Username] = u
					just_usernames = append(just_usernames, u.Username)
				}
			}
		}
	}
	log.Printf("Gathered %d users: %s", len(users), just_usernames)
	return users
}

func user_exists(username string) bool {
	var cmd *exec.Cmd
	cmd = exec.Command("id", username)
	if _, err := cmd.CombinedOutput(); err == nil {
		return true
	} else {
		return false
	}
}

func update_user(username string, attrs User) bool {
	log.Printf("Updating user: %s", username)
	var cmd *exec.Cmd
	var list []string
	list = append(list, "usermod -G '' "+username)
	for _, group := range attrs.Groups {
		list = append(list, "adduser "+username+" "+group)
	}
	list = append(list, "adduser "+username+" userd")
	for n, command := range list {
		log.Printf("Updating user, running cmd %d: %s", n, command)
		parts := strings.Fields(command)
		tail := parts[1:len(parts)]
		cmd = exec.Command(parts[0], tail...)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't update user: %s: %s", username, err)
			return false
		}
	}
	return true
}

func create_user(username string, attrs User) bool {
	log.Printf("Creating user: %s", username)
	var list []string
	list = append(list, "adduser --home /home/"+username+" --shell /bin/bash --disabled-password "+username)
	for _, group := range attrs.Groups {
		if group_exists(group) {
			list = append(list, "adduser "+username+" "+group)
		} else {
			log.Printf("Error: Can't add user %s to group %s, group doesn't exist", username, group)
		}
	}
	list = append(list, "adduser "+username+" userd")

	var cmd *exec.Cmd
	for n, command := range list {
		log.Printf("Creating user, running cmd %d: %s", n, command)
		parts := strings.Fields(command)
		tail := parts[1:len(parts)]
		cmd = exec.Command(parts[0], tail...)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't create user: %s: %s", username, err)
			return false
		}
	}
	return true
}

func delete_user(username string) bool {
	log.Printf("Deleting user: %s", username)
	var cmd *exec.Cmd
	var list []string
	list = append(list, "deluser --remove-home "+username)
	list = append(list, "deluser "+username+" userd")
	for n, command := range list {
		log.Printf("Deleting user, running cmd %d: %s", n, command)
		parts := strings.Fields(command)
		tail := parts[1:len(parts)]
		cmd = exec.Command(parts[0], tail...)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't delete user: %s: %s", username, err)
			return false
		}
	}
	return true
}

// add public key to ~/.ssh/authorized_keys, over-writes existing public key file
func set_ssh_public_keys(username string, keys []string) bool {
	key_data := strings.Join(keys, "\n")
	log.Printf("Setting ssh keys for %s (...%s)", username, strings.TrimSpace(key_data[len(key_data)-50:]))
	var buffer bytes.Buffer
	buffer.WriteString(key_data)
	os.Mkdir("/home/"+username+"/.ssh", 0500)
	if err := ioutil.WriteFile("/home/"+username+"/.ssh/authorized_keys", buffer.Bytes(), 0400); err != nil {
		log.Printf("Error: Can't write authorized_keys file for user: %s: %s", username, err)
	}
	return true
}

func make_group(group string) {
	var cmd *exec.Cmd
	cmd = exec.Command("groupadd", group)
	if _, err := cmd.CombinedOutput(); err == nil {
		log.Printf("Created group: %s", group)
	} else {
		log.Fatalf("Unable to create group: %s, (%s)", group, err)
	}
}

func group_exists(group string) bool {
	var cmd *exec.Cmd
	cmd = exec.Command("getent", "group", group)
	if _, err := cmd.CombinedOutput(); err == nil {
		return true
	} else {
		return false
	}
}

func get_group_members(group string) (users []string) {
	var cmd *exec.Cmd
	cmd = exec.Command("getent", "group", group)
	if output, err := cmd.CombinedOutput(); err == nil {
		// output looks like "cdrom:x:24:ubuntu,admin,user3"
		s := strings.Split(strings.TrimSpace(string(output[:])), ":")[3]
		if s != "" {
			users = strings.Split(s, ",")
		}
	} else {
		log.Print("Error: ", err)
	}
	log.Printf("Gathered %d users that are already in userd: %s", len(users), users)
	return users
}

func get_ops() (string, string) {
	realm := flag.String("realm", "", "the instance's realm eg: red, green, shunter")
	repo := flag.String("repo", "", "git repo where users are stored")
	flag.Parse()
	return *realm, *repo
}

func main() {
	log.SetPrefix("userd ")

	realm, repo := get_ops()
	validate(realm, repo)
	pull_or_clone(repo, "/tmp/")

	users := gather_json_users(repo, "/tmp/")

	for username, info := range users {
		delete := true
		for _, r := range info.Realms {
			if r == realm {
				delete = false
				ok := false
				if user_exists(username) {
					ok = update_user(username, info)
				} else {
					ok = create_user(username, info)
				}
				if ok {
					set_ssh_public_keys(username, info.SSHKeys)
				}
				break
			}
		}
		if delete && user_exists(username) {
			delete_user(username)
		}
	}

	log.Print("Completed")
}
