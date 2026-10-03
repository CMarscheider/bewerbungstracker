package service_test

import (
	"bytes"
	"errors"
	"testing"

	"bewerbungsmanager/internal/domain"
	"bewerbungsmanager/internal/service"
)

// jpegHeader reicht für die Typ-Erkennung per http.DetectContentType.
var jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00rest")

func TestCVPhotoLifecycle(t *testing.T) {
	svc := newService(t)
	_, err := svc.GetCVPhoto(ctx)
	var nf *service.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("ohne Foto erwartet NotFoundError, bekommen %v", err)
	}

	if err := svc.SaveCVPhoto(ctx, jpegHeader); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetCVPhoto(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, jpegHeader) || got.UpdatedAt.IsZero() {
		t.Errorf("Foto = %d Bytes, UpdatedAt %v", len(got.Data), got.UpdatedAt)
	}

	if err := svc.DeleteCVPhoto(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteCVPhoto(ctx); err != nil {
		t.Fatalf("zweites Löschen soll nicht fehlschlagen: %v", err)
	}
	if _, err := svc.GetCVPhoto(ctx); !errors.As(err, &nf) {
		t.Fatalf("nach dem Löschen erwartet NotFoundError, bekommen %v", err)
	}
}

func TestSaveCVPhotoRejectsNonJPEG(t *testing.T) {
	svc := newService(t)
	for name, data := range map[string][]byte{
		"leer": nil,
		"png":  []byte("\x89PNG\r\n\x1a\n0000"),
		"text": []byte("hallo"),
	} {
		err := svc.SaveCVPhoto(ctx, data)
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Field != "photo" {
			t.Errorf("%s: erwartet ValidationError(photo), bekommen %v", name, err)
		}
	}
}
