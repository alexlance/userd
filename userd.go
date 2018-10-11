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
	"sort"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

const (
	addUserCommand        = `adduser --disabled-password %s`
	delUserCommand        = `deluser --remove-home %s`
	changeShellCommand    = `usermod --shell %s %s`
	changePasswordCommand = `usermod --password '%s' %s`
	changeHomeDirCommand  = `usermod --move-home --home %s %s`
	changeGroupsCommand   = `usermod --groups %s %s`
	changeCommentCommand  = `usermod --comment "%s" %s`
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

// initial checks, all systems go?
func validate(realm string, repo string) {
	if repo == "" {
		log.Fatal("Error: Empty argument --repo")
	}
	if realm == "" {
		log.Fatal("Error: Empty argument --realm")
	}
	if os.Geteuid() != 0 {
		log.Fatalf("Error: Bad user id (%d), must run as root", os.Geteuid())
	}
	for _, cmd := range []string{"adduser", "deluser", "usermod", "getent"} {
		if _, err := exec.LookPath(cmd); err != nil {
			log.Fatalf("Error: Command not found: %s", cmd)
		}
	}
}

// clone a git repo full of json users into memory
func gitClone(repo string) *git.Repository {
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:   repo,
		Depth: 1,
	})
	if err != nil {
		log.Fatal("git clone ", repo, ": Error: ", err)
	}
	log.Print("git clone ", repo)
	return r
}

// gather all the users together who are meant to be in this instance's realm
func gatherJSONUsers(repo string, r *git.Repository, realm string) map[string]User {
	users := make(map[string]User)
	ref, _ := r.Head()
	commit, _ := r.CommitObject(ref.Hash())
	tree, _ := commit.Tree()

	tree.Files().ForEach(func(f *object.File) error {
		var u User
		if len(f.Name) > 5 && strings.ToLower(f.Name[len(f.Name)-5:]) == ".json" {
			content, _ := f.Contents()
			compact := strings.Join(strings.Fields(content), "")
			if err := json.Unmarshal([]byte(content), &u); err != nil {
				log.Printf("%s: Error: Parse or type error in JSON: %s", f.Name, compact)
			} else if u.Username == "" {
				log.Printf("%s: Error: Missing 'username' in JSON: %s", f.Name, compact)
			} else {
				if u.Home == "" {
					u.Home = "/home/" + u.Username
				} else {
					u.Home = path.Clean(u.Home)
				}
				if u.Shell == "" {
					u.Shell = "/bin/bash"
				}
				// sort them now, to make string comparisons simpler later on
				sort.Strings(u.SSHKeys)
				u.Groups = removeInvalidGroups(u.Groups, u.Username, realm)
				users[u.Username] = u
			}
		}
		return nil
	})
	return users
}

func removeInvalidGroups(groups []string, username string, realm string) (goodGroups []string) {
	for _, g := range groups {
		// per realm groups, eg: sudo:realm1:realm2:realm3
		if gr := strings.Split(g, ":"); len(gr) > 1 {
			g = gr[0]
			if !inRangePattern(realm, gr[1:]) {
				continue
			}
		}
		// ignore user's primary group, shouldn't mess with that
		if g == username {
			continue
		}
		// only include groups that exist on this instance
		if _, err := user.LookupGroup(g); err == nil {
			goodGroups = append(goodGroups, g)
		}
	}
	sort.Strings(goodGroups)
	return goodGroups
}

func userExists(username string) bool {
	if _, err := user.Lookup(username); err == nil {
		return true
	}
	return false
}

func createUser(username string, attrs User) bool {
	log.Printf("Creating user: %s", username)
	var cmd *exec.Cmd

	// ensure directory containing homedir exists
	if _, err := os.Stat(path.Dir(attrs.Home)); err != nil {
		exec.Command("mkdir", "-p", path.Dir(attrs.Home)).Run()
	}

	cmd = exec.Command("/bin/sh", "-c", fmt.Sprintf(addUserCommand, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't create user: %s: %s", username, err)
		return false
	}

	updateHome(username, attrs.Home)
	updateShell(username, attrs.Shell)

	return true
}

func updateShell(username string, shell string) bool {
	log.Printf("Updating shell for %s to %s", username, shell)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(changeShellCommand, shell, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't update shell for %s: %s", username, err)
		return false
	}
	return true
}

func updatePassword(username string, password string) bool {
	log.Printf("Updating password for %s", username)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(changePasswordCommand, password, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't update password for %s: %s", username, err)
		return false
	}
	return true
}

func updateHome(username string, home string) bool {
	log.Printf("Updating home for %s to %s", username, home)
	if _, err := os.Stat(path.Dir(home)); err != nil {
		exec.Command("mkdir", "-p", path.Dir(home)).Run()
	}
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(changeHomeDirCommand, home, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't update home dir for %s: %s", username, err)
		return false
	}
	return true
}

func updateComment(username string, comment string) bool {
	log.Printf("Updating comment for %s to %s", username, comment)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(changeCommentCommand, comment, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't update comment for %s: %s", username, err)
		return false
	}
	return true
}

func updateUser(username string, attrs User) bool {
	outp, _ := exec.Command("getent", "shadow", username).CombinedOutput()
	currentPassword := strings.TrimSpace(strings.Split(string(outp), ":")[1])
	outs, _ := exec.Command("getent", "passwd", username).CombinedOutput()
	currentShell := strings.TrimSpace(strings.Split(string(outs), ":")[6])
	currentHome := strings.TrimSpace(strings.Split(string(outs), ":")[5])
	currentComment := strings.TrimSpace(strings.Split(string(outs), ":")[4])

	if attrs.Shell != currentShell {
		updateShell(username, attrs.Shell)
	}
	if attrs.Password != currentPassword {
		updatePassword(username, attrs.Password)
	}
	if attrs.Home != currentHome {
		updateHome(username, attrs.Home)
	}
	if attrs.Comment != currentComment {
		updateComment(username, attrs.Comment)
	}
	return true
}

func deleteUser(username string) bool {
	log.Printf("Deleting user: %s", username)
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(delUserCommand, username))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Error: Can't delete user: %s: %s", username, err)
		return false
	}
	return true
}

// add public key to ~/.ssh/authorized_keys, over-writes existing public key file
func setSSHPublicKeys(username string, attrs User) bool {
	keyFile := path.Join(attrs.Home, ".ssh", "authorized_keys")
	keyData := strings.Join(attrs.SSHKeys, "\n")

	fileData := []string{}
	if buf, err := ioutil.ReadFile(keyFile); err == nil {
		fileData = strings.Split(string(buf), "\n")
		sort.Strings(fileData)
	}

	if strings.Join(attrs.SSHKeys, ",") != strings.Join(fileData, ",") {
		tail := 0
		if len(keyData) > 50 {
			tail = len(keyData) - 50
		}
		log.Printf("Updating ssh keys for %s (...%s)", username, strings.TrimSpace(keyData[tail:]))
		var buffer bytes.Buffer
		buffer.WriteString(keyData)
		os.Mkdir(path.Join(attrs.Home, ".ssh"), 0700)
		if err := ioutil.WriteFile(keyFile, buffer.Bytes(), 0600); err != nil {
			log.Printf("Error: Can't write %s file for user %s: %s", keyFile, username, err)
		}
	}
	// os.Chown isn't working, not sure why, use native chown instead
	exec.Command("chown", "-R", username+":"+username, path.Join(attrs.Home, ".ssh")).Run()
	return true
}

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

func updateUsersGroups(username string, attrs User) bool {
	if len(attrs.Groups) > 0 {
		existingGroups := getUserGroups(username)
		if strings.Join(existingGroups, ",") != strings.Join(attrs.Groups, ",") {
			log.Printf("Updating user groups for %s: %s", username, attrs.Groups)
			cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(changeGroupsCommand, strings.Join(attrs.Groups, ","), username))
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Printf("Error: Can't update user's groups for %s: %s %s", username, err, output)
				return false
			}
		}
	}
	return true
}

func getOps() (string, string) {
	realm := flag.String("realm", "", "the instance's realm eg: red, green, shunter")
	repo := flag.String("repo", "", "git repo where users are stored")
	flag.Parse()
	return *realm, *repo
}

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
	log.SetPrefix("userd v1.9 ")

	realm, repo := getOps()
	validate(realm, repo)

	r := gitClone(repo)
	users := gatherJSONUsers(repo, r, realm)

	for username, info := range users {
		if inRangePattern(realm, info.Realms) {
			if !userExists(username) {
				createUser(username, info)
			}

			if userExists(username) {
				updateUser(username, info)
				updateUsersGroups(username, info)
				setSSHPublicKeys(username, info)
			}

		} else if userExists(username) {
			deleteUser(username)
		}
	}
}
