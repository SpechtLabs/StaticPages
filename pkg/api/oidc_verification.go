package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/sierrasoftworks/humane-errors-go"
)

// oidcVerificationTimeout bounds how long an upload waits for the configured
// OIDC providers to accept or reject its token.
const oidcVerificationTimeout = 10 * time.Second

// verification is one issuer's verdict on a token.
type verification struct {
	metadata *s3_client.PageIndexData
	err      humane.Error
}

func (r *RestApi) extractAndVerifyAuth(ctx context.Context, authHeader string) (*s3_client.PageIndexData, humane.Error) {
	ctx, span := r.tracer.Start(ctx, "restApi.extractAndVerifyAuth")
	defer span.End()

	rawToken, err := extractBearerToken(authHeader)
	if err != nil {
		return nil, err
	}

	issuerSet, err := collectUniqueIssuers(r.conf.Pages)
	if err != nil {
		return nil, err
	}

	return verifyAgainstIssuers(ctx, rawToken, issuerSet, r.now())
}

func extractBearerToken(authHeader string) (string, humane.Error) {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return "", humane.New("missing or invalid Authorization header", "Make sure the Authorization header is correctly formatted and try again.")
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func collectUniqueIssuers(pages []*config.Page) (map[string]config.ClaimMap, humane.Error) {
	issuerSet := make(map[string]config.ClaimMap)
	for _, page := range pages {
		issuer, err := page.Git.GetOidcIssuer()
		if err != nil {
			return nil, humane.Wrap(err, "failed to get OIDC issuer", "Check the git provider of every page in the configuration.")
		}

		claimMap, err := page.Git.GetOidcClaimMapping()
		if err != nil {
			return nil, humane.Wrap(err, "failed to get OIDC claim mapping", "Check the git provider of every page in the configuration.")
		}

		issuerSet[issuer] = claimMap // deduplicates issuers
	}
	return issuerSet, nil
}

// verifyAgainstIssuers asks every configured issuer to verify the token at
// once, and returns the metadata of the first that accepts it, dated date. The
// results channel holds a verdict from every issuer, so none of them blocks,
// and on return the canceled context stops the ones still talking to their
// provider before it waits for them.
func verifyAgainstIssuers(ctx context.Context, rawToken string, issuerSet map[string]config.ClaimMap, date time.Time) (*s3_client.PageIndexData, humane.Error) {
	if len(issuerSet) == 0 {
		return nil, humane.New("no OIDC issuer is configured", "Configure a git provider for at least one page.")
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithTimeout(ctx, oidcVerificationTimeout)
	defer cancel()

	results := make(chan verification, len(issuerSet))
	for issuer, claimMap := range issuerSet {
		wg.Go(func() {
			metadata, herr := verifyWithIssuer(ctx, rawToken, issuer, claimMap, date)
			results <- verification{metadata: metadata, err: herr}
		})
	}

	var firstErr humane.Error
	for range len(issuerSet) {
		select {
		case result := <-results:
			if result.err == nil {
				return result.metadata, nil
			}
			if firstErr == nil {
				firstErr = result.err
			}

		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, humane.New("OIDC verification timed out",
					fmt.Sprintf("None of the configured OIDC providers answered within %s; check that they are reachable.", oidcVerificationTimeout))
			}
			return nil, humane.Wrap(ctx.Err(), "context canceled while verifying token", "The client went away; retry the upload.")
		}
	}

	return nil, humane.Wrap(firstErr, "none of the configured OIDC providers accepted the token",
		"Make sure the upload runs in a repository and workflow whose OIDC token one of the configured pages accepts.")
}

// verifyWithIssuer verifies the token with one issuer and reads the published
// commit from its claims.
func verifyWithIssuer(ctx context.Context, rawToken, issuer string, claimMap config.ClaimMap, date time.Time) (*s3_client.PageIndexData, humane.Error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, humane.Wrap(err, "failed to initialize OIDC provider", fmt.Sprintf("Make sure %s is reachable and serves an OIDC discovery document.", issuer))
	}

	verifier := provider.VerifierContext(ctx, &oidc.Config{SkipClientIDCheck: true})
	idToken, err := verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, humane.Wrap(err, "failed to verify OIDC token", fmt.Sprintf("Make sure the token was issued by %s and hasn't expired.", issuer))
	}

	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, humane.Wrap(err, "failed to extract claims from token", "Make sure the token's payload is a JSON object.")
	}

	return metadataFromClaims(claims, claimMap, date)
}

// metadataFromClaims reads the repository, commit, branch and, if present,
// environment from the token's claims.
func metadataFromClaims(claims map[string]json.RawMessage, claimMap config.ClaimMap, date time.Time) (*s3_client.PageIndexData, humane.Error) {
	repository, herr := stringClaim(claims, claimMap[config.RepositoryClaim])
	if herr != nil {
		return nil, humane.Wrap(herr, "failed to extract repository claim", "Check the claim mapping of the page's git provider.")
	}

	commit, herr := stringClaim(claims, claimMap[config.CommitClaim])
	if herr != nil {
		return nil, humane.Wrap(herr, "failed to extract commit claim", "Check the claim mapping of the page's git provider.")
	}

	branch, herr := stringClaim(claims, claimMap[config.BranchClaim])
	if herr != nil {
		return nil, humane.Wrap(herr, "failed to extract branch claim", "Check the claim mapping of the page's git provider.")
	}
	branch, _ = strings.CutPrefix(branch, "refs/heads/")

	// The environment is optional: only deployments carry one.
	environment, _ := stringClaim(claims, claimMap[config.EnvironmentClaim])

	return s3_client.NewPageCommitMetadata(repository, commit, branch, environment, date), nil
}

// stringClaim returns the named claim, which must be a string.
func stringClaim(claims map[string]json.RawMessage, name string) (string, humane.Error) {
	raw, ok := claims[name]
	if !ok {
		return "", humane.New(fmt.Sprintf("the token has no %q claim", name), "Check the claim mapping of the page's git provider.")
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", humane.Wrap(err, fmt.Sprintf("the token's %q claim is not a string", name), "Check the claim mapping of the page's git provider.")
	}

	return value, nil
}
