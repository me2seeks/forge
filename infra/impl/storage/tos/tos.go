package tos

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"

	"github.com/me2seeks/forge/infra/contract/storage"
	"github.com/me2seeks/forge/infra/impl/storage/proxy"
	"github.com/me2seeks/forge/logs"
	"github.com/me2seeks/forge/prelude/conv"
)

type tosClient struct {
	client     *tos.ClientV2
	bucketName string
}

func New(ctx context.Context, ak, sk, bucketName, endpoint, region string) (storage.Storage, error) {
	t, err := getTosClient(ctx, ak, sk, bucketName, endpoint, region)
	if err != nil {
		return nil, err
	}
	// t.test()
	return t, nil
}

func getTosClient(ctx context.Context, ak, sk, bucketName, endpoint, region string) (*tosClient, error) {
	credential := tos.NewStaticCredentials(ak, sk)
	client, err := tos.NewClientV2(endpoint,
		tos.WithCredentials(credential), tos.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("new tos client failed, bucketName: %s, endpoint: %s, region: %s, err: %v", bucketName, endpoint, region, err)
	}

	t := &tosClient{
		client:     client,
		bucketName: bucketName,
	}

	// Create bucket
	err = t.CheckAndCreateBucket(ctx)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *tosClient) test() {
	// test list objects
	ctx := context.Background()
	t.ListObjects(ctx, "")

	// test upload
	objectKey := fmt.Sprintf("test-%s.txt", time.Now().Format("20060102150405"))
	err := t.PutObject(context.Background(), objectKey, []byte("hello world"))
	if err != nil {
		logs.CtxErrorf(context.Background(), "PutObject failed, objectKey: %s, err: %v", objectKey, err)
	}

	// test download
	content, err := t.GetObject(context.Background(), objectKey)
	if err != nil {
		logs.CtxErrorf(context.Background(), "GetObject failed, objectKey: %s, err: %v", objectKey, err)
	}

	logs.CtxInfof(context.Background(), "GetObject content: %s", string(content))

	// Test Get URL
	url, err := t.GetObjectUrl(context.Background(), objectKey)
	if err != nil {
		logs.CtxErrorf(context.Background(), "GetObjectUrl failed, objectKey: %s, err: %v", objectKey, err)
	}

	logs.CtxInfof(context.Background(), "GetObjectUrl url: %s", url)

	// test delete
	err = t.DeleteObject(context.Background(), objectKey)
	if err != nil {
		logs.CtxErrorf(context.Background(), "DeleteObject failed, objectKey: %s, err: %v", objectKey, err)
	}
}

func (t *tosClient) CheckAndCreateBucket(ctx context.Context) error {
	client := t.client
	bucketName := t.bucketName

	_, err := client.HeadBucket(ctx, &tos.HeadBucketInput{Bucket: bucketName})
	if err == nil {
		return nil // already exist
	}

	serverErr, ok := err.(*tos.TosServerError)
	if !ok {
		return err
	}

	if serverErr.StatusCode == http.StatusNotFound {
		// Bucket does not exist
		logs.CtxInfof(ctx, "Bucket not found.")
		resp, err := client.CreateBucketV2(ctx, &tos.CreateBucketV2Input{
			Bucket: bucketName,
			ACL:    enum.ACLPrivate,
		})

		logs.CtxInfof(ctx, "Bucket Create resp: %v, err: %v", conv.DebugJsonToStr(resp), err)
		return err
	}

	return err
}

func (t *tosClient) PutObject(ctx context.Context, objectKey string, content []byte, opts ...storage.PutOptFn) error {
	opts = append(opts, storage.WithObjectSize(int64(len(content))))
	return t.PutObjectWithReader(ctx, objectKey, bytes.NewReader(content), opts...)
}

func (t *tosClient) PutObjectWithReader(ctx context.Context, objectKey string, content io.Reader, opts ...storage.PutOptFn) error {
	client := t.client
	bucketName := t.bucketName

	option := storage.PutOption{}
	for _, opt := range opts {
		opt(&option)
	}

	input := &tos.PutObjectV2Input{
		PutObjectBasicInput: tos.PutObjectBasicInput{
			Bucket: bucketName,
			Key:    objectKey,
		},
		Content: content,
	}

	if option.ContentType != nil {
		input.ContentType = *option.ContentType
	}
	if option.ContentEncoding != nil {
		input.ContentEncoding = *option.ContentEncoding
	}
	if option.ContentDisposition != nil {
		input.ContentDisposition = *option.ContentDisposition
	}
	if option.ContentLanguage != nil {
		input.ContentLanguage = *option.ContentLanguage
	}
	if option.Expires != nil {
		input.Expires = *option.Expires
	}

	if option.ObjectSize > 0 {
		input.ContentLength = option.ObjectSize
	}

	_, err := client.PutObjectV2(ctx, input)

	return err
}

func (t *tosClient) GetObject(ctx context.Context, objectKey string) ([]byte, error) {
	client := t.client
	bucketName := t.bucketName

	// Download data to memory
	getOutput, err := client.GetObjectV2(ctx, &tos.GetObjectV2Input{
		Bucket:                  bucketName,
		Key:                     objectKey,
		ResponseContentType:     "application/json",
		ResponseContentEncoding: "deflate",
	})
	if err != nil {
		return nil, err
	}

	// logs.CtxDebugf(ctx, "GetObject resp: %v, err: %v", conv.DebugJsonToStr(getOutput), err)

	body, err := io.ReadAll(getOutput.Content)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (t *tosClient) DeleteObject(ctx context.Context, objectKey string) error {
	client := t.client
	bucketName := t.bucketName

	// Delete the specified object in the bucket
	_, err := client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{
		Bucket: bucketName,
		Key:    objectKey,
	})

	return err
}

func (t *tosClient) GetObjectUrl(ctx context.Context, objectKey string, opts ...storage.GetOptFn) (string, error) {
	client := t.client
	bucketName := t.bucketName

	output, err := client.PreSignedURL(&tos.PreSignedURLInput{
		HTTPMethod: enum.HttpMethodGet,
		Expires:    60 * 60 * 24,
		Bucket:     bucketName,
		Key:        objectKey,
	})
	if err != nil {
		return "", err
	}

	ok, proxyURL := proxy.CheckIfNeedReplaceHost(ctx, output.SignedUrl)
	if ok {
		return proxyURL, nil
	}

	return output.SignedUrl, nil
}

func (t *tosClient) ListObjectsPaginated(ctx context.Context, input *storage.ListObjectsPaginatedInput) (*storage.ListObjectsPaginatedOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}
	if input.PageSize <= 0 {
		return nil, fmt.Errorf("page size must be positive")
	}

	output, err := t.client.ListObjectsV2(ctx, &tos.ListObjectsV2Input{
		Bucket: t.bucketName,
		ListObjectsInput: tos.ListObjectsInput{
			MaxKeys: int(input.PageSize),
			Marker:  input.Cursor,
			Prefix:  input.Prefix,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list objects failed, err: %w", err)
	}

	files := make([]*storage.FileInfo, 0, len(output.Contents))
	for _, obj := range output.Contents {
		if obj.Size == 0 && strings.HasSuffix(obj.Key, "/") {
			logs.CtxDebugf(ctx, "[ListObjectsPaginated] skip dir: %s", obj.Key)
			continue
		}

		files = append(files, &storage.FileInfo{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Size:         obj.Size,
		})
	}

	return &storage.ListObjectsPaginatedOutput{
		Files:       files,
		Cursor:      output.NextMarker,
		IsTruncated: output.IsTruncated,
	}, nil
}

func (t *tosClient) ListObjects(ctx context.Context, prefix string) ([]*storage.FileInfo, error) {
	const (
		DefaultPageSize = 100
		MaxListObjects  = 10000
	)

	files := make([]*storage.FileInfo, 0, DefaultPageSize)
	cursor := ""

	for {
		output, err := t.ListObjectsPaginated(ctx, &storage.ListObjectsPaginatedInput{
			Prefix:   prefix,
			PageSize: DefaultPageSize,
			Cursor:   cursor,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects failed, prefix = %v, err: %v", prefix, err)
		}

		for _, object := range output.Files {
			logs.CtxDebugf(ctx, "key = %s, lastModified = %s, eTag = %s, size = %d", object.Key, object.LastModified, object.ETag, object.Size)
			files = append(files, object)
		}

		cursor = output.Cursor
		logs.CtxDebugf(ctx, "IsTruncated = %v, Cursor = %s", output.IsTruncated, output.Cursor)

		if len(files) >= MaxListObjects {
			logs.CtxErrorf(ctx, "[ListObjects] max list objects reached, total: %d", len(files))
			break
		}

		if !output.IsTruncated || output.Cursor == "" {
			break
		}
	}

	return files, nil
}
