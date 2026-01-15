package api

import (
	"context"
	"fmt"
	"io"
)

// GetFileStream resolves the file from IPFS
// Returns: Reader, ContentType (if known), Size (if known), error
func (s *Server) GetFileStream(ctx context.Context, cid string) (io.Reader, string, int64, error) {
	// 1. Get File Stream from IPFS
	reader, err := s.Node.GetFile(ctx, cid)
	if err != nil {
		return nil, "", 0, fmt.Errorf("file not found or invalid CID: %w", err)
	}

	var contentType string
	var size int64

	// Check local DB for mimetype and size
	var fileRec struct {
		MimeType string
		Size     int64
	}
	if err := s.DB.Table("files").Where("cid = ?", cid).Scan(&fileRec).Error; err == nil {
		contentType = fileRec.MimeType
		size = fileRec.Size
	}

	return reader, contentType, size, nil
}
