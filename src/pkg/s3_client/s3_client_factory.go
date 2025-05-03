package s3_client

import (
	"github.com/SpechtLabs/StaticPages/pkg/config"
	"github.com/sierrasoftworks/humane-errors-go"
	"github.com/spf13/viper"
)

func GetS3Client(domain string) (*S3PageClient, humane.Error) {
	var cfg config.StaticPagesConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, humane.Wrap(err, "Unable to parse cfg file", "Make sure the cfg file exists, is readable, and conforms to the format.")
	}

	for _, page := range cfg.Pages {
		if page.Domain.Is(domain) {
			return NewS3PageClient(page), nil
		}
	}

	return nil, humane.New("Unable to find domain in cfg file", "Make sure the domain is spelled correctly.")
}
