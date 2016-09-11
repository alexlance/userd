package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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

// the format of the /etc/passwd file (man 5 passwd)
type PasswdEntry struct {
	Username string
	Password string
	UID      int
	GID      int
	Comment  string
	Home     string
	Shell    string
}

// the format of the /etc/group file (man 5 group)
type GroupEntry struct {
	Groupname string
	Password  string
	GID       int
	Users     []string
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
		log.Fatal("Error: Not installed: git")
	}
}

// second pass validation for later
func validate2(len_groups int, len_users int, len_entries int, len_entries_userd int, groups map[string]GroupEntry) {
	if len_groups < 1 {
		log.Fatal("No groups found")
	}
	if len_users < 1 {
		log.Fatal("No users in json found")
	}
	if len_entries < 1 {
		log.Fatal("No users found")
	}
	if len_entries_userd < 1 {
		log.Fatal("No users in the userd group found")
	}
	if _, ok := groups["userd"]; ok == false {
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
							log.Printf("%s: OK %s is meant to be in %s realm", name, u.Username, realm)
							users[u.Username] = u
						}
					}
				}
			}
		}
	}
	return users
}

func gather_passwd_users(path string) (map[string]PasswdEntry, []string) {
	lines := get_file_lines(path)
	entries := make(map[string]PasswdEntry)
	orig_order := []string{}
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 7 {
			log.Fatal("Error: Bad number of fields in %s: %s", path, line)
		}
		orig_order = append(orig_order, fields[0])
		entries[fields[0]] = PasswdEntry{
			Username: fields[0],
			Password: fields[1],
			UID:      to_int(fields[2]),
			GID:      to_int(fields[3]),
			Comment:  fields[4],
			Home:     fields[5],
			Shell:    fields[6],
		}
	}
	return entries, orig_order
}

func gather_groups(path string) map[string]GroupEntry {
	lines := get_file_lines(path)
	entries := make(map[string]GroupEntry)
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) != 4 {
			log.Fatal("Error: Bad number of fields in %s: %s", path, line)
		}
		entries[fields[0]] = GroupEntry{
			Groupname: fields[0],
			Password:  "<password>",
			GID:       to_int(fields[2]),
			Users:     strings.Split(fields[3], ","),
		}
	}
	return entries
}

func get_file_lines(path string) (lines []string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Error: Can't open %s", path)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lines = []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal("Error: ", err)
	}
	return lines
}

// we only care about user accounts that are in the userd group
func users_in_userd_group(entries map[string]PasswdEntry, users []string) (entries_userd map[string]PasswdEntry) {
	entries_userd = make(map[string]PasswdEntry)
	for _, v1 := range entries {
		for _, v2 := range users {
			if v2 == v1.Username {
				entries_userd[v1.Username] = v1
			}
		}
	}
	return
}

func user_exists(username string) bool {
	cmd := &exec.Cmd{}
	log.Printf("Checking user exists: %s", username)
	cmd = exec.Command("id", username)
	if _, err := cmd.CombinedOutput(); err == nil {
		return true
	} else {
		return false
	}
}

func update_user(username string, attrs map[string]string) bool {
	return true
}
func create_user(username string, attrs map[string]string) bool {
	return true
}
func delete_user(username string, attrs map[string]string) bool {
	return true
}

func main() {
	log.SetPrefix("userd ")

	realm := flag.String("realm", "", "the instance's realm")
	repo := flag.String("repo", "", "git repo where users are stored")
	_ = flag.Bool("force", false, "")
	flag.Parse()

	validate(*realm, *repo, 1000)
	pull_or_clone(*repo, "/tmp/")

	users := gather_json_users(*repo, "/tmp/", *realm)
	groups := gather_groups("/etc/group")
	entries, orig_order := gather_passwd_users("/etc/passwd")
	entries_userd := users_in_userd_group(entries, groups["userd"].Users)

	validate2(len(groups), len(users), len(entries), len(entries_userd), groups)

	log.Printf("Gathered %d groups from /etc/group", len(groups))
	log.Printf("Gathered %d users from git repo json", len(users))
	log.Printf("Gathered %d users from /etc/passwd", len(entries))
	log.Printf("Gathered %d users that are in group userd", len(entries_userd))

	// need to handle
	// adding new users
	// removing existing users
	// modifying existing users

	// loop through system users
	// loop through json, if user was existing userd user, then they retain UID
	// else give em json defined values

	// write down all pre-existing user accounts, except the userd managed users

	done := make(map[string]int)
	var buffer bytes.Buffer
	for _, username := range orig_order {
		v := entries[username]
		_, is_userd := entries_userd[username]
		if is_userd == false {
			buffer.WriteString(fmt.Sprintf("%s:%s:%d:%d:%s:%s:%s\n", v.Username, v.Password, v.UID, v.GID, v.Comment, v.Home, v.Shell))
			done[username] = 1
		}
	}

	buffer.WriteString("end system accounts\n")

	// if the userd user was already on the box, they should retain the same uid
	id := 8800
	for username, j := range users {
		if existing, ok := entries_userd[username]; ok == true {
			buffer.WriteString(fmt.Sprintf("%s:%s:%d:%d:%s (pre-existing,userd):%s:%s\n",
				existing.Username, j.Password, existing.UID, existing.GID, j.Comment, existing.Home, existing.Shell))

		} else if _, d := done[username]; d == false {
			buffer.WriteString(fmt.Sprintf("%s:%s:%d:%d:%s (userd):%s:%s\n",
				j.Username, j.Password, id, id, j.Comment, "/home/"+j.Username, "/bin/bash"))
			id += 2
		}
	}

	if err := ioutil.WriteFile("/tmp/new_passwd", buffer.Bytes(), 0644); err != nil {
		log.Fatal("Error: ", err)
	}

	//type PasswdEntry struct {
	//	Username string
	//	Password string
	//	UID      int
	//	GID      int
	//	Comment  string
	//	Home     string
	//	Shell    string
	//}

	// ensure existing users retain their UID
	//orig_uids := make(map[string]int)
	//for _, v := range groups["userd"].Users {
	//	if _, ok := entries[v]; ok == true {
	//		orig_uids[v] = entries[v].UID
	//	}
	//}

	//log.Printf("userds: %v", entries_userd)

	// spit out a passwd file with every user except the people in the userd group
	//lines := get_file_lines("/etc/passwd")

	// remove all users from userd group
	// but get all of their UID and GIDs first

	// add all

	log.Print("Completed")
}
