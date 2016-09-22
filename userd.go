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
	"strings"
)

// structure of user definition json files
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
func git_clone_or_pull(repo string, dest string) {
	dir := path.Base(strings.Split(repo, " ")[0])
	var cmd *exec.Cmd
	if os.Chdir(path.Join(dest, dir)) == nil {
		log.Println("Running git pull")
		exec.Command("git", "reset", "--hard").Run()
		cmd = exec.Command("git", "pull")
	} else {
		log.Println("Running git clone")
		cmd = exec.Command("git", "clone", repo, path.Join(dest, dir))
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("Error: ", err)
	}
	log.Print("git: " + string(out))
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
					u.Groups = valid_groups
					users[u.Username] = u
					usernames = append(usernames, u.Username)
				}
			}
		}
	}
	log.Printf("Gathered %d users: %s", len(users), usernames)
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
	key_data := strings.Join(attrs.SSHKeys, "\n")
	log.Printf("Setting ssh keys for %s (...%s)", username, strings.TrimSpace(key_data[len(key_data)-50:]))
	var buffer bytes.Buffer
	buffer.WriteString(key_data)
	os.Mkdir("/home/"+username+"/.ssh", 0700)
	if err := ioutil.WriteFile("/home/"+username+"/.ssh/authorized_keys", buffer.Bytes(), 0600); err != nil {
		log.Printf("Error: Can't write ~/.ssh/authorized_keys file for user: %s: %s", username, err)
	}
	// os.Chown isn't working, not sure why, use native chown instead
	exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.ssh").Run()
	return true
}

func update_users_groups(username string, attrs User) bool {
	var cmd *exec.Cmd
	if len(attrs.Groups) > 0 {
		log.Printf("Updating user groups: %s: %s", username, attrs.Groups)
		cmd = exec.Command("usermod", "-G", strings.Join(attrs.Groups, ","), username)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Error: Can't update user's groups: %s: %s %s", username, err, output)
			return false
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
	log.SetPrefix("userd ")

	realm, repo := get_ops()
	validate(realm, repo)
	git_clone_or_pull(repo, "/tmp/")
	users := gather_json_users(repo, "/tmp/")

	for username, info := range users {
		if in_range(realm, info.Realms) || in_range("all", info.Realms) {
			if !user_exists(username) {
				create_user(username, info)
			}
			update_users_groups(username, info)
			set_ssh_public_keys(username, info)

		} else if user_exists(username) {
			delete_user(username)
		}
	}
}
