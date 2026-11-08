package adminapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/storage"

	// The local driver is always available; s3 and other drivers are
	// enabled by importing their packages.
	_ "github.com/abysstract/abysstract-admin/backend/storage/localstorage"
)

// storageMountPath is where drivers that serve HTTP themselves are mounted.
const storageMountPath = "/static/"

// StorageConfig maps the application configuration to the storage driver
// configuration used by OpenStorage.
//
// Without ADMIN_STORAGE_SIGNING_SECRET the local driver's signing key is
// derived from the JWT secret, so presigned uploads work without extra
// configuration and change when the JWT secret is rotated.
func StorageConfig(cfg adminconfig.Config) storage.Config {
	dir := cfg.UploadDir
	if dir == "" {
		dir = "./uploads"
	}
	secret := []byte(cfg.StorageSigningSecret)
	if len(secret) == 0 && cfg.JWTSecret != "" {
		mac := hmac.New(sha256.New, []byte(cfg.JWTSecret))
		mac.Write([]byte("abysstract-admin/storage/upload-signing"))
		secret = mac.Sum(nil)
	}
	return storage.Config{
		Driver:        cfg.StorageDriverOrDefault(),
		PublicBaseURL: cfg.StoragePublicBaseURL,
		LocalDir:      dir,
		SigningSecret: secret,
		S3: storage.S3Config{
			Bucket:       cfg.StorageS3Bucket,
			Region:       cfg.StorageS3Region,
			Endpoint:     cfg.StorageS3Endpoint,
			UsePathStyle: cfg.StorageS3PathStyle,
		},
	}
}

// OpenStorage opens the driver selected by cfg. Applications that share one
// storage between the admin application and their own code open it here and
// pass it as Options.Storage; they then own it and close it when the driver
// implements io.Closer.
func OpenStorage(ctx context.Context, cfg adminconfig.Config) (storage.Storage, error) {
	return storage.Open(ctx, StorageConfig(cfg))
}
