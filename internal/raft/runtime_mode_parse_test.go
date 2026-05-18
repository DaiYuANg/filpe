package raft

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInitialMemberSupportsAtSeparator(t *testing.T) {
	t.Parallel()

	replicaID, target, err := parseInitialMember("7@127.0.0.1:63007")
	if err != nil {
		t.Fatalf("parse initial member: %v", err)
	}
	if replicaID != 7 {
		t.Fatalf("replicaID = %d, want %d", replicaID, 7)
	}
	if target != "127.0.0.1:63007" {
		t.Fatalf("target = %q, want %q", target, "127.0.0.1:63007")
	}
}

func TestParseInitialMembersSupportsWhitespaceAndAtSeparator(t *testing.T) {
	t.Parallel()

	members, err := parseInitialMembers(" 1 = 127.0.0.1:63001 , 2@127.0.0.1:63002 , 3 = 127.0.0.1:63003 ")
	if err != nil {
		t.Fatalf("parse initial members: %v", err)
	}
	if got := len(members); got != 3 {
		t.Fatalf("members count = %d, want 3", got)
	}
	if members[2] != "127.0.0.1:63002" {
		t.Fatalf("members[2] = %q, want %q", members[2], "127.0.0.1:63002")
	}
}

func TestParseInitialMembersRejectsInvalidEntry(t *testing.T) {
	t.Parallel()

	if _, err := parseInitialMembers("1-127.0.0.1:63001"); err == nil {
		t.Fatal("expected invalid entry error")
	}
}

func TestParseInitialMembersRejectsDuplicateReplicaID(t *testing.T) {
	t.Parallel()

	if _, err := parseInitialMembers("1=127.0.0.1:63001,1=127.0.0.1:63002"); err == nil {
		t.Fatal("expected duplicate replica ID error")
	}
}

func TestHasRaftStateReturnsTrueForNonEmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "meta"), []byte("test"), 0o600); err != nil {
		t.Fatalf("write raft marker: %v", err)
	}
	ok, err := HasRaftState(dir)
	if err != nil {
		t.Fatalf("HasRaftState: %v", err)
	}
	if !ok {
		t.Fatal("expected raft state to exist")
	}
}
