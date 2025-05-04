package api

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/SpechtLabs/StaticPages/pkg/s3_client"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/sierrasoftworks/humane-errors-go"
)

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

	return verifyAgainstIssuers(ctx, rawToken, issuerSet)
}

func waitForAllErrors(errs <-chan humane.Error) <-chan humane.Error {
	done := make(chan humane.Error, 1)

	go func() {
		var first humane.Error
		for err := range errs {
			if first == nil {
				first = err
			}

			// drain all other errors silently
		}

		done <- first
		close(done)
	}()

	return done
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
			return nil, humane.Wrap(err, "failed to get OIDC issuer")
		}

		claimMap, err := page.Git.GetOidcClaimMapping()
		if err != nil {
			return nil, humane.Wrap(err, "failed to get OIDC claim mapping")
		}

		issuerSet[issuer] = claimMap // deduplicates issuers
	}
	return issuerSet, nil
}

func verifyAgainstIssuers(ctx context.Context, rawToken string, issuerSet map[string]config.ClaimMap) (*s3_client.PageIndexData, humane.Error) {
	var (
		wg         sync.WaitGroup
		metadataCh = make(chan *s3_client.PageIndexData, 1)
		errorCh    = make(chan humane.Error, len(issuerSet))
	)

	for issuer, claimMap := range issuerSet {
		issuer := issuer
		claimMap := claimMap

		wg.Add(1)
		go func() {
			defer wg.Done()

			provider, err := oidc.NewProvider(ctx, issuer)
			if err != nil {
				errorCh <- humane.Wrap(err, "failed to initialize OIDC provider")
				return
			}

			verifier := provider.VerifierContext(ctx, &oidc.Config{SkipClientIDCheck: true})
			idToken, err := verifier.Verify(ctx, rawToken)
			if err != nil {
				errorCh <- humane.Wrap(err, "failed to verify OIDC token")
				return
			}

			claims := make(map[string]interface{})
			if err := idToken.Claims(&claims); err != nil {
				errorCh <- humane.Wrap(err, "failed to extract claims from token")
				return
			}

			var (
				repository  string
				commit      string
				branch      string
				environment string
			)

			if r, ok := claims[claimMap[config.RepositoryClaim]].(string); ok {
				repository = r
			} else {
				errorCh <- humane.New("failed to extract repository claim")
			}

			if c, ok := claims[claimMap[config.CommitClaim]].(string); ok {
				commit = c
			} else {
				errorCh <- humane.New("failed to extract commit claim")
			}

			if b, ok := claims[claimMap[config.BranchClaim]].(string); ok {
				branch, _ = strings.CutPrefix(b, "refs/heads/")
			} else {
				errorCh <- humane.New("failed to extract branch claim")
			}

			if e, ok := claims[claimMap[config.EnvironmentClaim]].(string); ok {
				environment = e
			}

			metadataCh <- s3_client.NewPageCommitMetadata(
				repository,
				commit,
				branch,
				environment,
				time.Now(),
			)
		}()
	}

	go func() {
		wg.Wait()
		close(errorCh)
	}()

	select {
	case metadata := <-metadataCh:
		return metadata, nil

	case <-ctx.Done():
		return nil, humane.Wrap(ctx.Err(), "context cancelled while verifying token")

	case <-time.After(10 * time.Second):
		return nil, humane.New("OIDC verification timed out")

	case err := <-waitForAllErrors(errorCh):
		return nil, humane.Wrap(err, "none of the configured OIDC providers accepted the token")
	}
}
