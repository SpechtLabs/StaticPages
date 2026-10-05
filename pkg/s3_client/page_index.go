package s3_client

import (
	"time"

	"github.com/sierrasoftworks/humane-errors-go"
)

// PageIndex is a map[commit-sha]
type PageIndex map[string]*PageIndexData

// PageIndexData represents metadata associated with a specific page commit.
// It includes the environment, branch name, and the date of the commit.
type PageIndexData struct {
	Environment string    `yaml:"environment"`
	Branch      string    `yaml:"branch"`
	Date        time.Time `yaml:"date"`
	sha         string
	repository  string
}

// NewPageCommitMetadata records a commit published from the repository.
func NewPageCommitMetadata(repository, sha, branch, environment string, date time.Time) *PageIndexData {
	return &PageIndexData{
		repository:  repository,
		sha:         sha,
		Environment: environment,
		Branch:      branch,
		Date:        date,
	}
}

// Repository returns the repository the commit was published from.
func (m *PageIndexData) Repository() string {
	return m.repository
}

// SHA returns the commit's SHA.
func (m *PageIndexData) SHA() string {
	return m.sha
}

// GetBySHA retrieves metadata for a specific domain and commit SHA
func (c PageIndex) GetBySHA(sha string) (*PageIndexData, humane.Error) {
	entry, exists := c[sha]
	if !exists {
		return nil, humane.New("metadata not found in index",
			"Make sure the commit was uploaded to this page; the index only lists uploaded commits.")
	}

	return entry, nil
}

// GetLatestForBranch returns the metadata for the latest commit on a specific branch
func (c PageIndex) GetLatestForBranch(branch string) (string, *PageIndexData, humane.Error) {
	var latestSHA string
	var latestData *PageIndexData

	for sha, entry := range c {
		if entry.Branch == branch && (latestData == nil || entry.Date.After(latestData.Date)) {
			latestSHA = sha
			latestData = entry
		}
	}

	if latestData == nil {
		return "", nil, humane.New("branch not found in index",
			"Make sure a commit on this branch was uploaded to this page.")
	}

	return latestSHA, latestData, nil
}
