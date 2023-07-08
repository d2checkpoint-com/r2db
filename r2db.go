package r2db

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
)

type Client struct {
	client     *s3.Client
	bucketName string
}

func NewClient(accountId, accessKeyId, accessKeySecret, bucketName string) (*Client, error) {
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)

	return &Client{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (s *Client) KeyExists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *Client) UploadKey(ctx context.Context, key string, data interface{}) error {
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)

	err := enc.Encode(data)
	if err != nil {
		return err
	}

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buffer.Bytes()),
	})

	return err
}

func (s *Client) DeleteKey(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	return err
}

func (s *Client) DownloadKey(ctx context.Context, key string, data interface{}) error {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer func(Body io.ReadCloser, err error) error {
		err = Body.Close()
		if err != nil {
			return err
		}
		return err
	}(output.Body, err)

	dec := gob.NewDecoder(output.Body)
	err = dec.Decode(data)

	return err
}

func (s *Client) ListKeys(ctx context.Context) ([]string, error) {
	var files []string

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucketName),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			files = append(files, *obj.Key)
		}
	}
	return files, nil
}

func (s *Client) GetKeyMetadata(ctx context.Context, key string) (map[string]string, error) {
	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return head.Metadata, nil
}
