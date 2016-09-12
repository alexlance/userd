package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// instance's realm eg: "redzone" "identity-api" "shunter" whatever you like
// and a store of user accounts (github.com:lexer-users/)
//
// get all members of group
// get all desired members of group and their ssh keys
//
// change user accounts on box to reflect desired users, MAINTAINING USER-IDS for existing accounts
//  update ssh authorized keys for each user
// if zero user accounts, do nothing
// don't fuck with the root account
// only fuck with user accounts who are in the userd group
// make sure the users is still in the userd group (edit group file!)

// invocation: userd --realm identity --repo git@github.com:lexerdev/lexer-users

// structure consistent with chef users
type User struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	UID      int      `json:"uid"`
	GUI      int      `json:"gid"`
	Comment  string   `json:"comment"`
	Home     string   `json:"home"`
	Shell    string   `json:"shell"`
	Groups   []string `json:"groups"`
	Realms   []string `json:"realms"`
	SSHKeys  []string `json:"ssh_keys"`
}

// convert a string number into an integer, or if error: zero
func to_int(a string) int {
	i, err := strconv.Atoi(a)
	if err != nil {
		log.Printf("Error: couldn't convert %s to integer, coercing to zero", a, err)
		return 0
	}
	return i
}

// initial checks, all systems go?
func validate(realm string, repo string, userid int) {
	if realm == "" {
		log.Fatal("Error: Empty argument --realm")
	}
	if repo == "" {
		log.Fatal("Error: Empty argument --repo")
	}
	if os.Geteuid() != userid {
		log.Fatalf("Error: Bad user id (%d), must run as root", os.Geteuid())
	}
	if _, err := exec.LookPath("git"); err != nil {
		log.Fatal("Error: Command not found: git")
	}
	if _, err := exec.LookPath("getent"); err != nil {
		log.Fatal("Error: Command not found: getent (install libc-bin)")
	}
	if ok := group_exists("userd"); ok == false {
		log.Fatal("Error: The group 'userd' doesn't exist")
	}
}

// go and grab a git repo full of json users
func pull_or_clone(repo string, dest string) {
	dir := path.Base(strings.Split(repo, " ")[0])
	cmd := &exec.Cmd{}
	if os.Chdir(path.Join(dest, dir)) == nil {
		log.Println("Running git pull")
		cmd = exec.Command("echo", "git", "pull")
	} else {
		log.Println("Running git clone")
		cmd = exec.Command("echo", "git", "clone", repo, path.Join(dest, dir))
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatal("Error: ", string(out), err)
	}
}

// gather all the users together who are meant to be in this instance's realm
func gather_json_users(repo string, dest string, realm string) map[string]User {
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
				} else if len(u.Realms) == 0 {
					log.Printf("%s: Error: Missing 'realms' in JSON: %s", name, compact)
				} else {
					for _, r := range u.Realms {
						if r == realm {
							users[u.Username] = u
							just_usernames = append(just_usernames, u.Username)
						}
					}
				}
			}
		}
	}
	log.Printf("Gathered %d users for %s realm: %s", len(users), realm, just_usernames)
	return users
}

func user_exists(username string) bool {
	cmd := &exec.Cmd{}
	cmd = exec.Command("id", username)
	if _, err := cmd.CombinedOutput(); err == nil {
		return true
	} else {
		return false
	}
}

func update_user(username string, attrs User) bool {
	log.Printf("Updating user: %s", username)
	return true
}
func create_user(username string, attrs User) bool {
	log.Printf("Creating user: %s", username)
	return true
}
func delete_user(username string) bool {
	log.Printf("Deleting user: %s", username)
	return true
}

func group_exists(group string) bool {
	cmd := &exec.Cmd{}
	cmd = exec.Command("getent", "group", group)
	if _, err := cmd.CombinedOutput(); err == nil {
		return true
	} else {
		return false
	}
}

func get_group_members(group string) []string {
	cmd := &exec.Cmd{}
	cmd = exec.Command("getent", "group", group)
	if output, err := cmd.CombinedOutput(); err == nil {
		l := strings.Split(strings.TrimSpace(string(output[:])), ":")
		u := strings.Split(strings.Join(l[len(l)-1:], ""), ",")
		log.Printf("Gathered %d users that are in group userd: %s", len(u), u)
		return u
	} else {
		return make([]string, 0)
	}
}

func get_ops() (string, string) {
	realm := flag.String("realm", "", "the instance's realm")
	repo := flag.String("repo", "", "git repo where users are stored")
	flag.Parse()
	return *realm, *repo
}

func main() {
	log.SetPrefix("userd ")

	realm, repo := get_ops()
	validate(realm, repo, 1000)
	pull_or_clone(repo, "/tmp/")

	users := gather_json_users(repo, "/tmp/", realm)
	entries_userd := get_group_members("userd")

	// for all users in the userd group
	for _, username := range entries_userd {
		if user_exists(username) {
			// if they're no longer in repo/json, delete them
			if info, ok := users[username]; ok == false {
				delete_user(username)
				// else update
			} else {
				update_user(username, info)
			}
		}
	}

	// for all users in the json
	for username, info := range users {
		create := true
		for _, un := range entries_userd {
			if username == un {
				create = false
			}
		}
		// if that user isn't in the userd group, add that user
		if create == true && !user_exists(username) {
			create_user(username, info)
		}
	}

	log.Print("Completed")
}
