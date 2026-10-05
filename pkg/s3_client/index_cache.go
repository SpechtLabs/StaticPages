package s3_client

import (
	"context"
	"sync"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.uber.org/zap"
)

// metadataCache holds each page's index for a minute, so the proxy doesn't
// download it from the bucket for every request. It's built, and its expiry
// loop started, on first use.
var metadataCache = sync.OnceValue(newMetadataCache)

// GetPageMetadata returns the page's index of published commits, from the
// cache or, on a miss, from the page's bucket.
func GetPageMetadata(ctx context.Context, page *config.Page) (PageIndex, humane.Error) {
	cache := metadataCache()

	// Check in memory cache
	if index := cache.Get(page.Domain); index != nil {
		return index.Value(), nil
	}

	// In case of cache miss, we fetch the index from S3
	s3Client := NewS3PageClient(page)
	metadata, err := s3Client.DownloadPageIndex(ctx)
	if err != nil {
		return nil, humane.Wrap(err, "unable to get page metadata",
			"Make sure the bucket exists and you have access to it.",
			"Make sure the page index exists and you have access to it.",
		)
	}

	cache.Set(page.Domain, metadata, ttlcache.DefaultTTL)
	return metadata, nil
}

// InvalidatePageMetadata drops the page's cached index, so the next request
// reads the one an upload just wrote.
func InvalidatePageMetadata(page *config.Page) {
	metadataCache().Delete(page.Domain)
}

func newMetadataCache() *ttlcache.Cache[config.DomainScope, PageIndex] {
	cache := ttlcache.New[config.DomainScope, PageIndex](
		ttlcache.WithTTL[config.DomainScope, PageIndex](1 * time.Minute),
		// TODO(cedi): evaluate if touch on hit might cause problems before disabling
		// ttlcache.WithDisableTouchOnHit[config.DomainScope, PageIndex](),
	)

	// Set up some debug logging
	cache.OnInsertion(func(ctx context.Context, item *ttlcache.Item[config.DomainScope, PageIndex]) {
		otelzap.L().Ctx(ctx).Debug("Page metadata inserted", zap.String("domain", item.Key().String()))
	})

	cache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[config.DomainScope, PageIndex]) {
		switch reason {
		case ttlcache.EvictionReasonExpired:
			otelzap.L().Ctx(ctx).Debug("Page metadata expired", zap.String("domain", item.Key().String()))

		case ttlcache.EvictionReasonDeleted:
			otelzap.L().Ctx(ctx).Debug("Page metadata deleted", zap.String("domain", item.Key().String()))

		case ttlcache.EvictionReasonCapacityReached:
			otelzap.L().Ctx(ctx).Warn("Page metadata cache capacity reached", zap.String("domain", item.Key().String()))
		}
	})

	// starts automatic expired item deletion, for the life of the process
	go cache.Start()

	return cache
}
