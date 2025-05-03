package s3_client

import (
	"time"

	"github.com/sierrasoftworks/humane-errors-go"
)

type PageMetadata map[string]*PageCommitMetadata

// PageCommitMetadata represents metadata associated with a specific page commit.
// It includes the environment, branch name, and the date of the commit.
type PageCommitMetadata struct {
	Environment string    `yaml:"environment"`
	Branch      string    `yaml:"branch"`
	Date        time.Time `yaml:"date"`
}

// PageEntry represents a single page's metadata in the cache
type PageEntry struct {
	Domain     string
	CommitSHA  string
	Metadata   *PageCommitMetadata
	Updated    time.Time
	SyncedToS3 bool
}

// DomainBranchIndex tracks the latest commit for each branch in a domain
type DomainBranchIndex struct {
	// Maps branch name to commit SHA
	Branches map[string]string
	// Maps branch name to latest commit timestamp
	LatestTimestamps map[string]time.Time
}

// GetBySHA retrieves metadata for a specific domain and commit SHA
func (c PageMetadata) GetBySHA(sha string) (*PageCommitMetadata, humane.Error) {
	entry, exists := c[sha]
	if !exists {
		return nil, humane.New("metadata not found in cache")
	}

	return entry, nil
}

// GetLatestForBranch returns the metadata for the latest commit on a specific branch
func (c PageMetadata) GetLatestForBranch(branch string) (string, *PageCommitMetadata, humane.Error) {
	for sha, entry := range c {
		if entry.Branch == branch {
			return sha, entry, nil
		}
	}

	return "", nil, humane.New("branch not found in cache")
}
