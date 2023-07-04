package git

import (
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	baseDir string
)

// GetBaseDir returns the base directory of the git repository
func GetBaseDir() (string, error) {
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine current working directory: %w", err)
		}
	}
	return baseDir, nil
}

func checkBehindMaster(appInterfaceDir string) error {
	fmt.Printf("### Checking 'master' branch is up to date ###\n")

	repo, err := git.PlainOpen(appInterfaceDir)
	if err != nil {
		return fmt.Errorf("failed to open git repository: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to retrieve head: %w", err)
	}
	if head.Name() != plumbing.Master {
		return fmt.Errorf("you are not on the 'master' branch")
	}

	// Fetch the latest changes from the upstream repository
	err = repo.Fetch(&git.FetchOptions{RemoteName: "upstream"})
	if err != nil {
		return fmt.Errorf("failed to fetch 'upstream' remote: %w", err)
	}

	commits, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve commit history: %W", err)
	}

	commits.ForEach(func(c *object.Commit) error {
		return nil
	})

	

//	cmd = exec.Command("git", "rev-list", "--count", "HEAD..upstream/master")
//	cmd.Dir = BaseDir
//	output, err = cmd.Output()
//	if err != nil {
//		return fmt.Errorf("error executing git rev-list command: %v", err)
//	}

//	behindCount := strings.TrimSpace(string(output))
//	if behindCount != "0" {
//		return fmt.Errorf("you are behind 'master' by this many commits: %s", behindCount)
//	}
	fmt.Printf("### 'master' branch is up to date ###\n\n")

	return nil
}
