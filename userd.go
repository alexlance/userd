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
	"sort"
	"strings"
)

// structure of user definition json files
type User struct {
	Username string   `json:"username"`
	Comment  string   `json:"comment"`
	Password string   `json:"password"`
	Shell    string   `json:"shell"`
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
	for _, cmd := range []string{"adduser", "deluser", "usermod", "git", "id", "getent", "groups"} {
		if _, err := exec.LookPath(cmd); err != nil {
			log.Fatalf("Error: Command not found: %s", cmd)
		}
	}
}

// go and grab a git repo full of json users
func git_clone(repo string, dest string) {
	dir := path.Base(strings.Split(repo, " ")[0])
	var cmd *exec.Cmd

	if path.Join(dest, dir) == path.Join(dest) {
		log.Fatal("Error: exiting, can't get the base name of ", repo, " dir: ", dir)
	}

	// remove repo if already exists, this is cleaner than pulling
	if _, err := os.Stat(path.Join(dest, dir)); err == nil {
		exec.Command("rm", "-rf", path.Join(dest, dir)).Run()
	}

	cmd = exec.Command("git", "clone", repo, path.Join(dest, dir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("git clone ", repo, ": Error: ", err)
	}
	exec.Command("chmod", "-R", "700", path.Join(dest, dir)).Run()
	log.Print("git clone ", repo, ": ", string(out))
}

// gather all the users together who are meant to be in this instance's realm
func gather_json_users(repo string, dest string) map[string]User {
	dir := path.Base(strings.Split(repo, " ")[0])
	files, err := ioutil.ReadDir(path.Join(dest, dir))
	if err != nil {
		log.Fatalf("Error: Can't read dir: %s %s", path.Join(dest, dir), err)
	}
	users := make(map[string]User)
	usernames := []string{}
	for _, f := range files {
		fname := f.Name()
		if f.IsDir() == false && len(fname) > 5 && strings.ToLower(fname[len(fname)-5:]) == ".json" {
			content, err := ioutil.ReadFile(path.Join(dest, dir, fname))
			if err != nil {
				log.Printf("Error: Trouble reading file: %s %s", fname, err)
			} else {
				var u User
				compact := strings.Join(strings.Fields(string(content)), "")
				if err := json.Unmarshal(content, &u); err != nil {
					log.Printf("%s: Error: Parse or type error in JSON: %s", fname, compact)
				} else if u.Username == "" {
					log.Printf("%s: Error: Missing 'username' in JSON: %s", fname, compact)
				} else {
					valid_groups := []string{}
					for _, g := range u.Groups {
						// only include groups that exist on this instance
						if exec.Command("getent", "group", g).Run() == nil {
							valid_groups = append(valid_groups, g)
						}
					}
					// sort them now, to make string comparisons simpler later on
					sort.Strings(u.SSHKeys)
					sort.Strings(valid_groups)
					u.Groups = valid_groups
					users[u.Username] = u
					usernames = append(usernames, u.Username)
				}
			}
		}
	}
	// log.Printf("Gathered %d users: %s", len(users), usernames)
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

func create_user(username string, attrs User) bool {
	log.Printf("Creating user: %s", username)
	var cmd *exec.Cmd
	cmd = exec.Command("adduser", "--home", "/home/"+username, "--shell", "/bin/bash", "--disabled-password", username)
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't create user: %s: %s", username, err)
		return false
	}
	return true
}

func update_user(username string, attrs User) bool {
	var cmd *exec.Cmd

	outp, _ := exec.Command("getent", "shadow", username).CombinedOutput()
	current_password := strings.TrimSpace(strings.Split(string(outp), ":")[1])
	outs, _ := exec.Command("getent", "passwd", username).CombinedOutput()
	current_shell := strings.TrimSpace(strings.Split(string(outs), ":")[6])

	if attrs.Shell != current_shell {
		log.Printf("Updating shell for %s to %s", username, attrs.Shell)
		cmd = exec.Command("usermod", "--shell", attrs.Shell, username)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't update shell for %s: %s", username, err)
			return false
		}
	}
	if attrs.Password != current_password {
		log.Printf("Updating password for %s to %s", username, attrs.Password)
		cmd = exec.Command("usermod", "--password", attrs.Password, username)
		if _, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't update password for %s: %s", username, err)
			return false
		}
	}
	return true
}

func delete_user(username string) bool {
	log.Printf("Deleting user: %s", username)
	var cmd *exec.Cmd
	cmd = exec.Command("deluser", "--remove-home", username)
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't delete user: %s: %s", username, err)
		return false
	}
	return true
}

// add public key to ~/.ssh/authorized_keys, over-writes existing public key file
func set_ssh_public_keys(username string, attrs User) bool {
	key_file := "/home/" + username + "/.ssh/authorized_keys"
	key_data := strings.Join(attrs.SSHKeys, "\n")

	file_data := []string{}
	if buf, err := ioutil.ReadFile(key_file); err == nil {
		file_data = strings.Split(string(buf), "\n")
		sort.Strings(file_data)
	}

	if strings.Join(attrs.SSHKeys, ",") != strings.Join(file_data, ",") {
		tail := 0
		if len(key_data) > 50 {
			tail = len(key_data) - 50
		}
		log.Printf("Setting ssh keys for %s (...%s)", username, strings.TrimSpace(key_data[tail:]))
		var buffer bytes.Buffer
		buffer.WriteString(key_data)
		os.Mkdir("/home/"+username+"/.ssh", 0700)
		if err := ioutil.WriteFile(key_file, buffer.Bytes(), 0600); err != nil {
			log.Printf("Error: Can't write %s file for user %s: %s", key_file, username, err)
		}
	}
	// os.Chown isn't working, not sure why, use native chown instead
	exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.ssh").Run()
	return true
}

func update_users_groups(username string, attrs User) bool {
	var cmd *exec.Cmd
	if len(attrs.Groups) > 0 {
		cmd = exec.Command("groups", username)
		if output, err := cmd.CombinedOutput(); err == nil {
			o := strings.TrimSpace(string(output))
			o = strings.Replace(o, username+" : "+username+" ", "", 1)
			existingGroups := strings.Split(o, " ")
			sort.Strings(existingGroups)

			if strings.Join(existingGroups, ",") != strings.Join(attrs.Groups, ",") {
				log.Printf("Updating user groups for %s: %s", username, attrs.Groups)
				cmd = exec.Command("usermod", "-G", strings.Join(attrs.Groups, ","), "--comment", attrs.Comment, username)
				if output, err := cmd.CombinedOutput(); err != nil {
					log.Printf("Error: Can't update user's groups for %s: %s %s", username, err, output)
					return false
				}
			}
		}
	}
	return true
}

func get_ops() (string, string) {
	realm := flag.String("realm", "", "the instance's realm eg: red, green, shunter")
	repo := flag.String("repo", "", "git repo where users are stored")
	flag.Parse()
	return *realm, *repo
}

func in_range(needle string, haystack []string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func main() {
	log.SetPrefix("userd v1.3 ")

	realm, repo := get_ops()
	validate(realm, repo)
	git_clone(repo, "/etc/")
	users := gather_json_users(repo, "/etc/")

	for username, info := range users {
		if in_range(realm, info.Realms) || in_range("all", info.Realms) {
			if !user_exists(username) {
				create_user(username, info)
			}

			if user_exists(username) {
				update_user(username, info)
				update_users_groups(username, info)
				set_ssh_public_keys(username, info)
			}

		} else if user_exists(username) {
			delete_user(username)
		}
	}
}
