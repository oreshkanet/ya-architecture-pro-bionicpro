package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const storageLogPrefix = "[reports-backend][storage]"

func UserIDHashPrefix(userID string) string {
	h := sha256.Sum256([]byte(userID))
	return hex.EncodeToString(h[:])[:16]
}

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3(endpoint, bucket, accessKey, secretKey string, reportTTLDays int) (*S3Storage, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			SigningRegion:     "us-east-1",
			HostnameImmutable: true,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	st := &S3Storage{client: client, bucket: bucket}
	if err := st.EnsureBucket(context.Background()); err != nil {
		log.Printf("%s EnsureBucket: %v", storageLogPrefix, err)
		return nil, err
	}
	if reportTTLDays > 0 {
		if err := st.EnsureBucketLifecycle(context.Background(), reportTTLDays); err != nil {
			log.Printf("%s EnsureBucketLifecycle: %v", storageLogPrefix, err)
			return nil, err
		}
	}
	log.Printf("%s S3 connected bucket=%s", storageLogPrefix, bucket)
	return st, nil
}

func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
		_, createErr := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
		if createErr != nil {
			return fmt.Errorf("create bucket %s: %w", s.bucket, createErr)
		}
		log.Printf("%s bucket created: %s", storageLogPrefix, s.bucket)
		return nil
	}
	return fmt.Errorf("head bucket %s: %w", s.bucket, err)
}

func (s *S3Storage) EnsureBucketLifecycle(ctx context.Context, reportTTLDays int) error {
	if reportTTLDays <= 0 {
		return nil
	}
	_, err := s.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(s.bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("expire-reports"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{
						Prefix: aws.String("reports/"),
					},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(int32(reportTTLDays)),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket lifecycle %s: %w", s.bucket, err)
	}
	log.Printf("%s bucket lifecycle set: reports/ expire after %d days", storageLogPrefix, reportTTLDays)
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var respErr *awshttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if contentType == "" {
		contentType = "text/html; charset=utf-8"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(body),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=300"), // 5 мин для ревалидации после ETL
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}
