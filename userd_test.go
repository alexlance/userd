package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"testing"
)

func TestUserExists(t *testing.T) {
	valid := userExists("eddie")
	if !valid {
		t.Error("User eddie is missing")
	}
}

func TestCreateAndDeleteUser(t *testing.T) {
	user := User{
		Home: "/volume1/home/gary",
	}
	valid := createUser("gary", user)
	if !valid {
		t.Error("User gary is missing")
	}

	if !veirfyUser("gary") || !veirfyGroup("gary") {
		t.Error("Cannot find gary User and Group")
	}

	valid = deleteUser("gary")

	if !valid {
		t.Error("Unable to delete user gary")
	}

	if veirfyUser("gary") || veirfyGroup("gary") {
		t.Error("User/Group gary still exists")
	}
}

func TestUpdateGroup(t *testing.T) {
	groups := []string{"sudo"}
	valid := updateGroups("eddie", groups)
	if !valid {
		t.Error("Unable to update group")
	}
}

func TestUpdateComment(t *testing.T) {
	valid := updateComment("eddie", "hare are you")
	if !valid {
		t.Error("Unable to update comment")
	}
}

func TestUpdatePassword(t *testing.T) {
	valid := updatePassword("eddie", "password123")
	if !valid {
		t.Error("Unable to update password")
	}
}

func TestUpdateHome(t *testing.T) {
	valid := updatePassword("eddie", "/volume1/home/eddie2")
	if !valid {
		t.Error("Unable to update home dir")
	}
}

func TestUpdateSSHPublicKeys(t *testing.T) {
	u := User{
		Home:    "/volume1/home/gary",
		SSHKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOb0cAFpJJTKtXt8xekQg6uqREamVz8wFGRb8TBrKJ9Y root@25ae4bddbf60"},
	}
	valid := updateSSHPublicKeys("eddie", u)
	if !valid {
		t.Error("Unable to update ssh keys")
	}
}

func TestGetUserGroups(t *testing.T) {
	groups := getUserGroups("eddie")
	if len(groups) != 1 {
		t.Errorf("User eddie should have 1 groups but got %v", len(groups))
	}
}

//Helper functions
func veirfyUser(u string) bool {
	_, err := user.Lookup(u)
	if err != nil {
		return false
	}
	return true
}

func veirfyGroup(u string) bool {
	_, err := user.LookupGroup(u)
	if err != nil {
		return false
	}
	return true
}

func Setup() {
	//create user
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(addUserCommand, "eddie"))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Error: Can't create user: %s: %s", "eddie", err)
	}
}

func Destroy() {
	//create user
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(delUserCommand, "eddie"))
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Error: Can't delete user: %s: %s", "eddie", err)
	}
}

func TestMain(m *testing.M) {
	Setup()
	retCode := m.Run()
	Destroy()
	os.Exit(retCode)
}
