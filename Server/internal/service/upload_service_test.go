package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
)

func makePNG(t *testing.T) (*multipart.FileHeader, int) {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	return makeUpload(t, "img.png", "image/png", buf.Bytes()), buf.Len()
}

func makeUpload(t *testing.T, name, ct string, data []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	hdr.Set("Content-Type", ct)
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = io.Copy(part, bytes.NewReader(data))
	require.NoError(t, w.Close())
	r := multipart.NewReader(body, w.Boundary())
	form, err := r.ReadForm(10 << 20)
	require.NoError(t, err)
	return form.File["file"][0]
}

func TestUploadService_AcceptsPNG(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 5 << 20, AllowedMIME: []string{"image/png", "image/jpeg", "image/webp"},
	})
	fh, _ := makePNG(t)
	url, err := svc.SaveImage(context.Background(), fh)
	require.NoError(t, err)
	require.Contains(t, url, "/uploads/")
	files, _ := os.ReadDir(dir)
	require.Len(t, files, 1)
}

func TestUploadService_RejectsTooBig(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 100, AllowedMIME: []string{"image/png"},
	})
	big := make([]byte, 200)
	fh := makeUpload(t, "x.png", "image/png", big)
	_, err := svc.SaveImage(context.Background(), fh)
	require.Error(t, err)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeFileTooLarge, ae.Code)
}

func TestUploadService_RejectsWrongMIME(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 5 << 20, AllowedMIME: []string{"image/png"},
	})
	payload := []byte("MZ\x90\x00" + string(make([]byte, 64)))
	fh := makeUpload(t, "evil.png", "image/png", payload)
	_, err := svc.SaveImage(context.Background(), fh)
	require.Error(t, err)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeFileType, ae.Code)
	files, _ := os.ReadDir(dir)
	require.Empty(t, files)
}
