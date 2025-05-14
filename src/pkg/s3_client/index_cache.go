package s3_client

import (
	"context"
	"time"

	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/jellydator/ttlcache/v3"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spechtlabs/go-otel-utils/otelzap"
	"go.uber.org/zap"
)

var (
	_metadataCache *ttlcache.Cache[config.DomainScope, PageIndex]
)

func init() {
	_metadataCache = ttlcache.New[config.DomainScope, PageIndex](
		ttlcache.WithTTL[config.DomainScope, PageIndex](1 * time.Minute),
		// TODO: evaluate if touch on hit might cause problems before disabling
		// ttlcache.WithDisableTouchOnHit[config.DomainScope, PageIndex](),
	)

	// Set up some debug logging
	_metadataCache.OnInsertion(func(ctx context.Context, item *ttlcache.Item[config.DomainScope, PageIndex]) {
		otelzap.L().Ctx(ctx).Debug("Page metadata inserted", zap.String("domain", item.Key().String()))
	})

	_metadataCache.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, item *ttlcache.Item[config.DomainScope, PageIndex]) {
		switch reason {
		case ttlcache.EvictionReasonExpired:
			otelzap.L().Ctx(ctx).Debug("Page metadata expired", zap.String("domain", item.Key().String()))
			break

		case ttlcache.EvictionReasonDeleted:
			otelzap.L().Ctx(ctx).Debug("Page metadata deleted", zap.String("domain", item.Key().String()))
			break

		case ttlcache.EvictionReasonCapacityReached:
			otelzap.L().Ctx(ctx).Warn("Page metadata cache capacity reached", zap.String("domain", item.Key().String()))
			break
		}
	})

	// starts automatic expired item deletion
	go _metadataCache.Start()
}

func GetPageMetadata(ctx context.Context, page *config.Page) (PageIndex, error) {
	// Check in memory cache
	if index := _metadataCache.Get(page.Domain); index != nil {
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

	_metadataCache.Set(page.Domain, metadata, ttlcache.DefaultTTL)
	return metadata, nil
}

func InvalidatePageMetadata(page *config.Page) {
	_metadataCache.Delete(page.Domain)
}
