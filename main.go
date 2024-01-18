package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

// User account modelled in a json file
type User struct {
	Username string   `json:"username"`
	Comment  string   `json:"comment"`
	Password string   `json:"password"`
	Shell    string   `json:"shell"`
	Home     string   `json:"home"`
	Groups   []string `json:"groups"`
	Realms   []string `json:"realms"`
	SSHKeys  []string `json:"ssh_keys"`
}

// vars that are global-ish
var (
	debug  bool
	realm  string
	repo   string
	distro distroCommands
)

// grab the command line arguments and figure out which OS we're on
func init() {
	log.SetPrefix("userd v1.21 ")
	if os.Geteuid() != 0 {
		log.Fatalf("Error: Bad user id (%d), must run as root", os.Geteuid())
	}

	flag.StringVar(&realm, "realm", "", "the instance's realm eg: dev, stage, prod")
	flag.StringVar(&repo, "repo", "", "git repo where users are stored")
	flag.BoolVar(&debug, "debug", false, "print debugging info")
	flag.Parse()

	if realm == "" {
		log.Fatal("Error: Empty argument --realm")
	}
	if repo == "" {
		log.Fatal("Error: Empty argument --repo")
	}

	v := getOS()
	if v == "" {
		log.Fatal("Unable to detect operating system")
	}
	distro = getOSCommands(v)
}

// for debugging
func info(msg string) {
	if debug {
		log.Printf("DEBUG: %s", msg)
	}
}

// clone a git repo full of json users into memory
func gitClone(repo string) *object.FileIter {
	log.Print("git clone ", repo)
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:   repo,
		Depth: 1,
	})
	if err != nil {
		log.Fatal("git clone ", repo, ": Error: ", err)
	}
	ref, err := r.Head()
	if err != nil {
		log.Fatal("git clone ", repo, " Can't get head: Error: ", err)
	}
	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		log.Fatal("git clone ", repo, " Can't get commit: Error: ", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		log.Fatal("git clone ", repo, " Can't get files: Error: ", err)
	}
	return tree.Files()
}

// gather all the users together who are meant to be in this instance's realm
func gatherUsers(files *object.FileIter) (users []User) {
	files.ForEach(func(f *object.File) error {
		var u User
		if len(f.Name) > 5 && strings.ToLower(f.Name[len(f.Name)-5:]) == ".json" {
			content, _ := f.Contents()
			compact := strings.Join(strings.Fields(content), "")
			err := json.Unmarshal([]byte(content), &u)
			if err != nil {
				log.Printf("%s: Error: Parse or type error in JSON: %s", f.Name, compact)
			} else if u.Username == "" {
				log.Printf("%s: Error: Missing 'username' in JSON: %s", f.Name, compact)
			} else {
				if u.Home == "" {
					u.Home = path.Clean("/home/" + u.Username)
				} else {
					u.Home = path.Clean(u.Home)
				}
				if u.Shell == "" {
					u.Shell = "/bin/bash"
				} else {
					u.Shell = path.Clean(u.Shell)
				}
				// sort them now, to make string comparisons simpler later on
				sort.Strings(u.SSHKeys)
				users = append(users, u)
			}
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	return users
}

// check the groups that are available on this system
func removeInvalidGroups(u *User, realm string) {
	// the groups field can contain realm-specific group membership rules, eg:
	//
	// "groups" : [
	//   "audio",               # user belongs in the audio group for all of their realms
	//   "video:realm1",        # user belongs in the video group, but only in realm1
	//   "spidey:realm1:realm2" # user belongs in the spidey group, but only in realm1 and realm2
	//  ]
	goodGroups := []string{}
	for _, g := range u.Groups {

		// if group:realm format
		if gr := strings.Split(g, ":"); len(gr) > 1 {
			if !inRangePattern(realm, gr[1:]) {
				continue
			}
			g = gr[0]
		}

		// don't include user's primary group
		if g == u.Username {
			continue
		}

		// only include groups that exist on this instance
		if _, err := user.LookupGroup(g); err != nil {
			continue
		}

		goodGroups = append(goodGroups, g)
	}
	sort.Strings(goodGroups)
	u.Groups = goodGroups
}

// check if a user account exists on this system
func userExists(username string) bool {
	if _, err := user.Lookup(username); err == nil {
		return true
	}
	return false
}

// create a new user account
func createUser(u User) bool {
	log.Printf("Creating user: %s", u.Username)
	if out, err := distro.addUser(u.Username, u.Home); err != nil {
		log.Printf("Error: Can't create user: %s: %s %s", u.Username, err, out)
		return false
	}
	return true
}

// delete a user account
func deleteUser(username string) bool {
	log.Printf("Deleting user: %s", username)
	if out, err := distro.delUser(username); err != nil {
		log.Printf("Error: Can't delete user: %s: %s %s", username, err, out)
		return false
	}
	return true
}

// update the details of an existing user account
func updateUser(u User) bool {
	output_pass, _ := exec.Command("getent", "shadow", u.Username).CombinedOutput()
	if pass := strings.Split(string(output_pass), ":"); len(pass) > 1 {
		if u.Password != strings.TrimSpace(pass[1]) {
			updatePassword(u.Username, u.Password)
		}
	} else {
		log.Printf("Error: Can't get password hash for user: %s", u.Username)
	}

	output_details, _ := exec.Command("getent", "passwd", u.Username).CombinedOutput()
	if details := strings.Split(string(output_details), ":"); len(details) > 6 {
		if u.Shell != strings.TrimSpace(details[6]) {
			updateShell(u.Username, u.Shell)
		}
		if u.Home != strings.TrimSpace(details[5]) {
			updateHome(u.Username, u.Home)
		}
		if toAlphNum(u.Comment) != strings.TrimSpace(details[4]) {
			updateComment(u.Username, toAlphNum(u.Comment))
		}
	} else {
		log.Printf("Error: Can't get user details for user: %s", u.Username)
	}

	existingGroups := getUserGroups(u.Username)
	if strings.Join(existingGroups, ",") != strings.Join(u.Groups, ",") {
		updateGroups(u.Username, u.Groups)
	}

	keyFile := path.Join(u.Home, ".ssh", "authorized_keys")
	fileData := []string{}
	if buf, err := ioutil.ReadFile(keyFile); err == nil {
		fileData = strings.Split(string(buf), "\n")
		sort.Strings(fileData)
	}
	if strings.Join(u.SSHKeys, ",") != strings.Join(fileData, ",") {
		updateSSHPublicKeys(u.Username, u)
	}
	return true
}

// change user's default shell
func updateShell(username string, shell string) bool {
	log.Printf("Updating shell for %s to %s", username, shell)
	if out, err := distro.changeShell(username, shell); err != nil {
		log.Printf("Error: Can't update shell for %s: %s %s", username, err, out)
		return false
	}
	return true
}

// change users password
func updatePassword(username string, password string) bool {
	log.Printf("Updating password for %s", username)
	info(fmt.Sprintf("New password: %s", password))
	if out, err := distro.changePassword(username, password); err != nil {
		log.Printf("Error: Can't update password for %s: %s %s", username, err, out)
		return false
	}
	return true
}

// change users home directory
func updateHome(username string, home string) bool {
	log.Printf("Updating home dir for %s to %s", username, home)
	if out, err := distro.changeHomeDir(username, home); err != nil {
		log.Printf("Error: Can't update home dir for %s: %s %s", username, err, out)
		return false
	}
	return true
}

// change users gecos comment
func updateComment(username string, comment string) bool {
	log.Printf("Updating comment for %s to %s", username, comment)
	if out, err := distro.changeComment(username, comment); err != nil {
		log.Printf("Error: Can't update comment for %s: %s %s", username, err, out)
		return false
	}
	return true
}

// strip out characters that are problematic with passwd files
func toAlphNum(input string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9 ]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(input, "")
}

// get the list of groups a user belongs to
func getUserGroups(username string) (groups []string) {
	u, _ := user.Lookup(username)
	gids, _ := u.GroupIds()
	for _, gid := range gids {
		group, _ := user.LookupGroupId(gid)
		if group.Name != username { // ignore the user's primary group (same name as username)
			groups = append(groups, group.Name)
		}
	}
	sort.Strings(groups)
	return groups
}

// change a users list of groups they belong to
func updateGroups(username string, groups []string) bool {
	log.Printf("Updating user groups for %s: %s", username, groups)
	if out, err := distro.changeGroups(username, strings.Join(groups, ",")); err != nil {
		log.Printf("Error: Can't update user's groups for %s: %s %s", username, err, out)
		return false
	}
	return true
}

// update the user's ~/.ssh/authorized_keys file with their public keys
func updateSSHPublicKeys(username string, u User) bool {
	keyFile := path.Join(u.Home, ".ssh", "authorized_keys")
	keyData := strings.Join(u.SSHKeys, "\n")
	tail := 0
	if len(keyData) > 50 {
		tail = len(keyData) - 50
	}
	log.Printf("Updating ssh keys for %s (...%s)", username, strings.TrimSpace(keyData[tail:]))
	info(keyData)
	var buffer bytes.Buffer
	buffer.WriteString(keyData)
	os.Mkdir(path.Join(u.Home, ".ssh"), 0700)
	if err := ioutil.WriteFile(keyFile, buffer.Bytes(), 0600); err != nil {
		log.Printf("Error: Can't write %s file for user %s: %s", keyFile, username, err)
	}
	// os.Chown isn't working, not sure why, use native chown instead
	exec.Command("chown", "-R", username+":"+username, path.Join(u.Home, ".ssh")).Run()
	return true
}

// search for a close match in a range
func inRangePattern(needle string, haystack []string) bool {
	for _, v := range haystack {
		// filepath.Match performs glob/wildcard matching
		if match, _ := filepath.Match(v, needle); match || v == needle {
			return true
		}
	}
	return false
}

func main() {
	files := gitClone(repo)
	users := gatherUsers(files)

	for _, u := range users {
		if inRangePattern(realm, u.Realms) {
			if userExists(u.Username) || createUser(u) {
				removeInvalidGroups(&u, realm)
				updateUser(u)
			}
		} else if userExists(u.Username) {
			deleteUser(u.Username)
		}
	}
}
