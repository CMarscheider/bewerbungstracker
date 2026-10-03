package httpapi_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

var jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00rest")

func putPhoto(t *testing.T, srv *httptest.Server, data []byte) response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/cv/photo", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return response{Status: res.StatusCode, ContentType: res.Header.Get("Content-Type"), Body: body}
}

func TestCVPhotoUploadDownloadDelete(t *testing.T) {
	srv := newTestServer(t)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil), http.StatusNotFound, "/problems/not-found")

	expectStatus(t, putPhoto(t, srv, jpegHeader), http.StatusNoContent)

	got := call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil)
	expectStatus(t, got, http.StatusOK)
	if got.ContentType != "image/jpeg" || !bytes.Equal(got.Body, jpegHeader) {
		t.Errorf("GET = %s, %d Bytes", got.ContentType, len(got.Body))
	}

	expectStatus(t, call(t, srv, http.MethodDelete, "/api/v1/cv/photo", nil), http.StatusNoContent)
	expectProblem(t, call(t, srv, http.MethodGet, "/api/v1/cv/photo", nil), http.StatusNotFound, "/problems/not-found")
}

func TestCVPhotoRejectsNonJPEG(t *testing.T) {
	srv := newTestServer(t)
	p := expectProblem(t, putPhoto(t, srv, []byte("kein bild")), http.StatusBadRequest, "/problems/validation-error")
	if p["field"] != "photo" {
		t.Errorf("field = %v", p["field"])
	}
}

func TestCVPhotoTooLargeIsBadRequest(t *testing.T) {
	srv := newTestServer(t)
	big := append(append([]byte{}, jpegHeader...), make([]byte, 2<<20)...)
	r := putPhoto(t, srv, big)
	if r.Status != http.StatusBadRequest && r.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("Status %d, erwartet 400 oder 413; Body %s", r.Status, r.Body)
	}
	t.Logf("zu großer Upload: Status %d, Body %s", r.Status, r.Body)
}
