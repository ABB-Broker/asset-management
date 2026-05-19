package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

const (
	// BaseURL is prepended when serving stored photo paths to the frontend.
	// Change this when deploying to production (e.g. https://your-domain.com).
	BaseURL = "http://localhost:2005"

	// UploadRoot is the on-disk root for all uploaded files.
	// The Fiber static middleware should serve this at /uploads.
	UploadRoot = "./uploads"
)

// SaveFile writes a multipart upload to:
//
//	<UploadRoot>/<entityUUID>/<subdir>/<randomhex><ext>
//
// and returns the relative path that should be stored in the database:
//
//	<entityUUID>/<subdir>/<randomhex><ext>
//
// subdir should be "rooms" or "assets" depending on the entity type.
func SaveFile(fh *multipart.FileHeader, entityUUID, subdir string) (string, error) {
	dir := filepath.Join(UploadRoot, entityUUID, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir %q: %w", dir, err)
	}

	// Random 8-byte hex filename to avoid collisions and path traversal.
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate filename entropy: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	filename := hex.EncodeToString(b) + ext

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open upload source: %w", err)
	}
	defer src.Close()

	dstPath := filepath.Join(dir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("create destination file %q: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("write upload: %w", err)
	}

	// Always use forward slashes in the stored path regardless of OS.
	return entityUUID + "/" + subdir + "/" + filename, nil
}

// DeleteFile removes the on-disk file for a stored relative path.
// Errors are silently swallowed because a missing file should never
// block a database delete operation.
func DeleteFile(relativePath string) {
	if relativePath == "" {
		return
	}
	_ = os.Remove(filepath.Join(UploadRoot, filepath.FromSlash(relativePath)))
}

// WithBaseURL converts a stored relative path (e.g. "abc-123/rooms/deadbeef.png")
// to a full URL the frontend can use (e.g. "http://localhost:3000/uploads/abc-123/rooms/deadbeef.png").
// Returns an empty string unchanged so templates can safely call it on any value.
func WithBaseURL(relativePath string) string {
	if relativePath == "" {
		return ""
	}
	return BaseURL + "/uploads/" + relativePath
}
