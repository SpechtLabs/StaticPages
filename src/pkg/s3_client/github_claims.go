package s3_client

import "strings"

type GitHubClaims struct {
	Repository  string `json:"repository"`
	Sha         string `json:"sha"`
	Environment string `json:"environment"`
	Ref         string `json:"ref"`
	Actor       string `json:"actor"`
	Event       string `json:"event"`
	Workflow    string `json:"workflow"`
	Job         string `json:"job"`
	RunID       string `json:"run_id"`
	RunNumber   string `json:"run_number"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
}

func (g *GitHubClaims) Branch() string {
	branch, _ := strings.CutPrefix(g.Ref, "refs/heads/")
	return branch
}
