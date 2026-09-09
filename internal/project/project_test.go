package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeUsesCommonRepositoryIdentity(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "repo")
	gitDir := filepath.Join(root, ".git")
	worktreeGitDir := filepath.Join(gitDir, "worktrees", "feature")
	worktree := filepath.Join(directory, "feature")
	if err := os.MkdirAll(worktreeGitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktree, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[remote \"origin\"]\n\turl = git@github.com:example/project.git\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+worktreeGitDir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mainProject := Resolve(root)
	worktreeProject := Resolve(worktree)
	if mainProject.ID != worktreeProject.ID {
		t.Fatalf("main ID %q differs from worktree ID %q", mainProject.ID, worktreeProject.ID)
	}
	if mainProject.Identity != "git@github.com/example/project" {
		t.Fatalf("identity = %q", mainProject.Identity)
	}
}
