package helpers

import (
	"fmt"
	"log"
	"os"

	"github.com/go-git/go-git/v5"
)

// CloneRepo clones github repo locally.
// This is required if there is no mcm container image tag supplied or
// the clusters are not seed (control) and shoot (target) clusters
func CloneRepo(source string, destinationDir string) error {
	fi, err := os.Stat(destinationDir)
	if err == nil {
		if fi.IsDir() {
			log.Printf(
				"skipping as %s directory already exists. If cloning is necessary, delete directory and rerun test",
				destinationDir)
			return nil
		}
	}

	fmt.Println("Cloning Repository ...")
	// clone the given repository to the given directory
	fmt.Printf("git clone %s %s --recursive", source, destinationDir)

	repo, err := git.PlainClone(destinationDir,
		false,
		&git.CloneOptions{
			URL:               source,
			RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		},
	)
	if err != nil {
		fmt.Printf("\nFailed to clone repoistory to the destination; %s.\n", destinationDir)
		return err
	}

	// retrieving the branch being pointed by HEAD
	ref, err := repo.Head()
	if err != nil {
		return err
	}

	// retrieving the commit object
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return err
	}

	fmt.Println(commit)

	return nil
}
