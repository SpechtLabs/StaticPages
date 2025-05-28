package s3_client_test

import (
	"testing"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/stretchr/testify/assert"
)

func TestPageIndex_GetBySHA(t *testing.T) {
	now := time.Now()

	index := s3_client.PageIndex{
		"abc123": s3_client.NewPageCommitMetadata("repo1", "abc123", "main", "prod", now),
	}

	tests := []struct {
		name     string
		sha      string
		expected *s3_client.PageIndexData
		isErr    bool
	}{
		{
			name:     "SHA exists in index",
			sha:      "abc123",
			expected: index["abc123"],
			isErr:    false,
		},
		{
			name:     "SHA does not exist in index",
			sha:      "notfound",
			expected: nil,
			isErr:    true,
		},
		{
			name:     "Empty SHA key",
			sha:      "",
			expected: nil,
			isErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := index.GetBySHA(tt.sha)
			if tt.isErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPageIndex_GetLatestForBranch(t *testing.T) {
	base := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	index := s3_client.PageIndex{
		"sha1": s3_client.NewPageCommitMetadata("repo1", "sha1", "main", "prod", base),
		"sha2": s3_client.NewPageCommitMetadata("repo1", "sha2", "main", "prod", base.Add(2*time.Hour)),
		"sha3": s3_client.NewPageCommitMetadata("repo1", "sha3", "dev", "staging", base.Add(1*time.Hour)),
	}

	tests := []struct {
		name        string
		branch      string
		expectedSHA string
		expected    *s3_client.PageIndexData
		isErr       bool
	}{
		{
			name:        "Get latest for main branch",
			branch:      "main",
			expectedSHA: "sha2",
			expected:    index["sha2"],
			isErr:       false,
		},
		{
			name:        "Get latest for dev branch",
			branch:      "dev",
			expectedSHA: "sha3",
			expected:    index["sha3"],
			isErr:       false,
		},
		{
			name:        "Branch does not exist",
			branch:      "feature/missing",
			expectedSHA: "",
			expected:    nil,
			isErr:       true,
		},
		{
			name:        "Empty branch name",
			branch:      "",
			expectedSHA: "",
			expected:    nil,
			isErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha, result, err := index.GetLatestForBranch(tt.branch)
			if tt.isErr {
				assert.Error(t, err)
				assert.Empty(t, sha)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSHA, sha)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Optional: Performance test (benchmark)
func BenchmarkPageIndex_GetLatestForBranch(b *testing.B) {
	index := s3_client.PageIndex{}
	branch := "main"
	start := time.Now()

	// Generate 100,000 entries with mixed branches
	for i := 0; i < 100000; i++ {
		sha := "sha" + time.Now().Add(time.Duration(i)*time.Second).Format("150405")
		entryBranch := branch
		if i%2 == 0 {
			entryBranch = "dev"
		}
		index[sha] = s3_client.NewPageCommitMetadata("repo", sha, entryBranch, "env", start.Add(time.Duration(i)*time.Second))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = index.GetLatestForBranch(branch)
	}
}
