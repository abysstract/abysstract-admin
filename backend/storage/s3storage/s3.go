// Package s3storage is the S3-compatible storage driver, registered as "s3".
// It works with AWS S3 and with services that speak the S3 API, such as
// MinIO and Cloudflare R2, through an endpoint override and path-style
// addressing.
//
// Importing this package pulls in the AWS SDK; applications that only need
// the local driver do not import it. Enable the driver for configuration-based
// setup with a blank import:
//
//	import _ "github.com/abysstract/abysstract-admin/backend/storage/s3storage"
//
// Credentials come from the standard AWS chain: environment variables
// (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY), shared config and credentials
// files, and instance or task roles.
package s3storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/abysstract/abysstract-admin/backend/storage"
)

// MaxPresignTTL is the longest validity S3 accepts for a presigned URL.
const MaxPresignTTL = 7 * 24 * time.Hour

func init() {
	storage.Register("s3", func(ctx context.Context, cfg storage.Config) (storage.Storage, error) {
		return New(ctx, Options{
			Bucket:        cfg.S3.Bucket,
			Region:        cfg.S3.Region,
			Endpoint:      cfg.S3.Endpoint,
			UsePathStyle:  cfg.S3.UsePathStyle,
			PublicBaseURL: cfg.PublicBaseURL,
		})
	})
}

// Options configures the S3 driver.
type Options struct {
	Bucket string
	// Region is required by request signing. Cloudflare R2 uses "auto".
	Region string
	// Endpoint overrides the service URL, for example
	// "http://localhost:9000" for MinIO.
	Endpoint string
	// UsePathStyle addresses objects as endpoint/bucket/key. MinIO usually
	// needs it.
	UsePathStyle bool
	// PublicBaseURL prefixes keys in URL, for a CDN or a public bucket
	// domain. Empty derives the bucket URL from Endpoint or the AWS region.
	PublicBaseURL string
	// Credentials overrides the default credential chain.
	Credentials aws.CredentialsProvider
	// Now overrides the clock used for ExpiresAt in tests.
	Now func() time.Time
}

// Storage is the S3 driver.
type Storage struct {
	client    *s3.Client
	presign   *s3.PresignClient
	bucket    string
	publicURL string
	now       func() time.Time
}

var _ storage.Storage = (*Storage)(nil)

// New loads the AWS configuration and creates the driver.
func New(ctx context.Context, opts Options) (*Storage, error) {
	if err := checkOptions(opts); err != nil {
		return nil, err
	}
	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(opts.Region)}
	if opts.Credentials != nil {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(opts.Credentials))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3storage: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if opts.Endpoint != "" {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		}
		o.UsePathStyle = opts.UsePathStyle
		// Checksums only when the operation requires them: many
		// S3-compatible services reject the newer default trailers, and
		// presigned uploads from browsers cannot compute them.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return NewWithClient(client, opts)
}

// NewWithClient wraps an existing client. Only Bucket, Region, Endpoint,
// UsePathStyle, PublicBaseURL and Now are read from opts.
func NewWithClient(client *s3.Client, opts Options) (*Storage, error) {
	if client == nil {
		return nil, errors.New("s3storage: client is nil")
	}
	if err := checkOptions(opts); err != nil {
		return nil, err
	}
	publicURL := opts.PublicBaseURL
	if publicURL == "" {
		publicURL = defaultPublicURL(opts)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Storage{
		client:    client,
		presign:   s3.NewPresignClient(client),
		bucket:    opts.Bucket,
		publicURL: publicURL,
		now:       now,
	}, nil
}

func checkOptions(opts Options) error {
	if strings.TrimSpace(opts.Bucket) == "" {
		return errors.New("s3storage: Bucket is required")
	}
	if strings.TrimSpace(opts.Region) == "" {
		return errors.New("s3storage: Region is required")
	}
	if opts.Endpoint != "" {
		u, err := url.Parse(opts.Endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("s3storage: Endpoint %q must be an http(s) URL", opts.Endpoint)
		}
	}
	return nil
}

func defaultPublicURL(opts Options) string {
	if opts.Endpoint != "" {
		u, _ := url.Parse(opts.Endpoint)
		if opts.UsePathStyle {
			return strings.TrimSuffix(opts.Endpoint, "/") + "/" + url.PathEscape(opts.Bucket) + "/"
		}
		return u.Scheme + "://" + opts.Bucket + "." + u.Host + "/"
	}
	if opts.UsePathStyle {
		return "https://s3." + opts.Region + ".amazonaws.com/" + url.PathEscape(opts.Bucket) + "/"
	}
	return "https://" + opts.Bucket + ".s3." + opts.Region + ".amazonaws.com/"
}

// Client returns the underlying S3 client for operations this package does
// not wrap.
func (s *Storage) Client() *s3.Client { return s.client }

// Bucket returns the bucket name.
func (s *Storage) Bucket() string { return s.bucket }

// URL implements storage.Storage.
func (s *Storage) URL(key string) string { return storage.JoinURL(s.publicURL, key) }

// Put implements storage.Storage. Readers that cannot seek are sent with an
// unsigned payload, since the payload hash cannot be computed in advance.
func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if err := storage.ValidateKey(key); err != nil {
		return err
	}
	in := &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: r}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	var optFns []func(*s3.Options)
	if _, ok := r.(io.ReadSeeker); !ok {
		optFns = append(optFns, s3.WithAPIOptions(v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware))
	}
	if _, err := s.client.PutObject(ctx, in, optFns...); err != nil {
		return fmt.Errorf("s3storage: put %s: %w", key, err)
	}
	return nil
}

// Stat implements storage.Storage.
func (s *Storage) Stat(ctx context.Context, key string) (storage.ObjectInfo, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.ObjectInfo{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return storage.ObjectInfo{}, fmt.Errorf("%w: %s", storage.ErrNotFound, key)
		}
		return storage.ObjectInfo{}, fmt.Errorf("s3storage: head %s: %w", key, err)
	}
	info := storage.ObjectInfo{
		Key:         key,
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}
	if out.LastModified != nil {
		info.LastModified = *out.LastModified
	}
	return info, nil
}

func isNotFound(err error) bool {
	var nf *types.NotFound
	var nsk *types.NoSuchKey
	if errors.As(err, &nf) || errors.As(err, &nsk) {
		return true
	}
	var re *awshttp.ResponseError
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}

// PresignPut implements storage.Storage. The signature covers the content
// type and length, so S3 rejects a body of another size or type. Headers
// lists what the client must send; Host and Content-Length are left out
// because browsers set them.
func (s *Storage) PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (storage.PresignedUpload, error) {
	if err := storage.ValidateKey(key); err != nil {
		return storage.PresignedUpload{}, err
	}
	if size <= 0 {
		return storage.PresignedUpload{}, fmt.Errorf("s3storage: presign size must be positive (got %d)", size)
	}
	if ttl <= 0 || ttl > MaxPresignTTL {
		return storage.PresignedUpload{}, fmt.Errorf("s3storage: presign ttl must be in (0, %s] (got %s)", MaxPresignTTL, ttl)
	}
	in := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentLength: aws.Int64(size),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	issued := s.now()
	req, err := s.presign.PresignPutObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return storage.PresignedUpload{}, fmt.Errorf("s3storage: presign %s: %w", key, err)
	}
	headers := make(map[string]string, len(req.SignedHeader))
	for name, values := range req.SignedHeader {
		canonical := http.CanonicalHeaderKey(name)
		if canonical == "Host" || canonical == "Content-Length" || len(values) == 0 {
			continue
		}
		headers[canonical] = strings.Join(values, ",")
	}
	return storage.PresignedUpload{
		URL:       req.URL,
		Method:    req.Method,
		Headers:   headers,
		ExpiresAt: issued.Add(ttl),
	}, nil
}
