// Package publish uploads a generated repository tree to S3-compatible object
// storage.
//
// One code path covers Cloudflare R2, MinIO, Backblaze B2 and Amazon S3.
// Nothing here special-cases a provider. See
// docs/decisions/0003-s3-compatible-storage.md.
//
// Upload order matters: pool objects and Packages indices are written before
// the Release files that reference them, so a client fetching mid-publish
// either sees the previous repository or the new one, never a Release
// promising content that has not landed.
package publish
