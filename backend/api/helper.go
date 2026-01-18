package api

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/boxo/files"
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
	} else {
		// Try shared history
		var sharedRec struct {
			MimeType string
			Size     int64
		}
		if err := s.DB.Table("shared_files").Where("cid = ?", cid).Scan(&sharedRec).Error; err == nil {
			contentType = sharedRec.MimeType
			size = sharedRec.Size
		}
	}

	// Fallback: If DB size is 0, try to get from reader if it supports Size()
	if size == 0 {
		if f, ok := reader.(files.File); ok {
			if s, err := f.Size(); err == nil {
				size = s
			}
		}
	}

	return reader, contentType, size, nil
}

// CopyFile copies a single file from src to dst
func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	return nil
}

// CopyDir copies a whole directory recursively
func CopyDir(src string, dst string) error {
	var err error
	var fds []os.DirEntry
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = os.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := filepath.Join(src, fd.Name())
		dstfp := filepath.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(srcfp, dstfp); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcfp, dstfp); err != nil {
				return err
			}
		}
	}
	return nil
}
