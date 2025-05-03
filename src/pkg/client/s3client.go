package client

import "github.com/SpechtLabs/StaticPages/pkg/config"

type S3Client struct {
	bucketConf config.BucketConfig
}

func NewS3Client(bucketConf config.BucketConfig) *S3Client {
	return &S3Client{
		bucketConf: bucketConf,
	}
}
