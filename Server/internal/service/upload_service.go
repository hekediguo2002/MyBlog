package service

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/idgen"
)

type UploadOptions struct {
	Dir         string
	URLPrefix   string
	MaxBytes    int64
	AllowedMIME []string
}

type UploadService struct {
	opt UploadOptions
}

func NewUploadService(opt UploadOptions) *UploadService {
	if opt.URLPrefix == "" {
		opt.URLPrefix = "/uploads"
	}
	if opt.MaxBytes == 0 {
		opt.MaxBytes = 5 << 20
	}
	return &UploadService{opt: opt}
}

func (s *UploadService) SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	if fh.Size > s.opt.MaxBytes {
		return "", apperr.New(apperr.CodeFileTooLarge, "文件过大")
	}
	src, err := fh.Open()
	if err != nil {
		return "", apperr.New(apperr.CodeInvalidParam, "无法读取上传文件")
	}
	defer src.Close()

	header := make([]byte, 512)
	n, _ := io.ReadFull(src, header)
	mime := http.DetectContentType(header[:n])
	if !contains(s.opt.AllowedMIME, mime) {
		return "", apperr.New(apperr.CodeFileType, "文件类型不支持")
	}

	ext := mimeToExt(mime)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(fh.Filename))
	}

	if err := os.MkdirAll(s.opt.Dir, 0o755); err != nil {
		return "", apperr.Wrap(apperr.CodeUnknown, "无法创建上传目录", err)
	}
	name := idgen.NewUUID() + ext
	dst := filepath.Join(s.opt.Dir, name)

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeUnknown, "写文件失败", err)
	}
	defer out.Close()

	full := io.MultiReader(strings.NewReader(string(header[:n])), src)
	limited := io.LimitReader(full, s.opt.MaxBytes+1)
	written, err := io.Copy(out, limited)
	if err != nil {
		_ = os.Remove(dst)
		return "", apperr.Wrap(apperr.CodeUnknown, "写文件失败", err)
	}
	if written > s.opt.MaxBytes {
		_ = os.Remove(dst)
		return "", apperr.New(apperr.CodeFileTooLarge, "文件过大")
	}
	return path.Join(s.opt.URLPrefix, name), nil
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

func mimeToExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
