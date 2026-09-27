package admin

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "golang.org/x/image/webp"
)

// testJPEG is a 600x1000 portrait: red band across the top third, blue
// below, so a crop's position is visible in the pixels.
func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 600, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 600; x++ {
			c := color.NRGBA{0, 0, 255, 255}
			if y < 333 {
				c = color.NRGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation inserts an APP1/EXIF segment carrying tag 0x0112.
func withOrientation(jpg []byte, o uint16) []byte {
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08") // big-endian, IFD0 at offset 8
	ifd := make([]byte, 2+12+4)
	binary.BigEndian.PutUint16(ifd, 1)
	binary.BigEndian.PutUint16(ifd[2:], 0x0112)
	binary.BigEndian.PutUint16(ifd[4:], 3) // SHORT
	binary.BigEndian.PutUint32(ifd[6:], 1)
	binary.BigEndian.PutUint16(ifd[10:], o)
	payload := append([]byte("Exif\x00\x00"), append(tiff, ifd...)...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append([]byte{0xFF, 0xD8}, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestExifOrientationApplied(t *testing.T) {
	src := withOrientation(testJPEG(t), 6)
	if got := exifOrientation(src); got != 6 {
		t.Fatalf("orientation read %d, want 6", got)
	}
	d, err := decodePhoto(src)
	if err != nil {
		t.Fatal(err)
	}
	if b := d.img.Bounds(); b.Dx() != 1000 || b.Dy() != 600 {
		t.Errorf("orientation 6 must rotate 600x1000 into 1000x600, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestCropFocus(t *testing.T) {
	src := image.Rect(0, 0, 600, 1000)
	top := cropFocus(src, 0.5, 0.0)
	bottom := cropFocus(src, 0.5, 1.0)
	if top.Min.Y != 0 || top.Dx() != 600 || top.Dy() != 750 {
		t.Errorf("top crop %v", top)
	}
	if bottom.Max.Y != 1000 || bottom.Min.Y != 250 {
		t.Errorf("bottom crop %v", bottom)
	}
	wide := cropFocus(image.Rect(0, 0, 1000, 500), 0.9, 0.5)
	if wide.Dx() != 400 || wide.Dy() != 500 || wide.Max.X != 1000 {
		t.Errorf("wide crop %v", wide)
	}
}

func (c *client) upload(target, field, name string, body []byte) *httptest.ResponseRecorder {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("_csrf", c.csrf)
	fw, _ := mw.CreateFormFile(field, name)
	_, _ = fw.Write(body)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, target, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = c.ip
	for k, v := range c.cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 {
			delete(c.cookies, ck.Name)
		} else {
			c.cookies[ck.Name] = ck.Value
		}
	}
	return rec
}

func TestPhotoPipeline(t *testing.T) {
	e := newEnv(t)
	c := e.client(t)
	c.loginOwner()
	c.get("/admin/coaches/c-gocha-butbaia")

	// HTML dressed as a JPEG is refused with a message, not stored.
	rec := c.upload("/admin/coaches/c-gocha-butbaia/photo", "photo", "evil.jpg", []byte("<html><script>alert(1)</script></html>"))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("bad upload: %d", rec.Code)
	}
	if body := c.get("/admin/coaches/c-gocha-butbaia").Body.String(); !strings.Contains(body, "не поддерживается") {
		t.Error("no error flash after a bad upload")
	}
	if files, _ := filepath.Glob(filepath.Join(e.st.UploadDir(), "c-gocha-butbaia-*")); len(files) != 0 {
		t.Errorf("bad upload left files: %v", files)
	}

	// A real portrait: derivatives exist at 400x500 and 800x1000, the
	// public card and page point at /uploads/, and the file is served.
	rec = c.upload("/admin/coaches/c-gocha-butbaia/photo", "photo", "IMG_0001.JPG", testJPEG(t))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	row, _ := e.st.CoachRow("c-gocha-butbaia")
	if row.PhotoFile == "" || !row.Photo {
		t.Fatalf("photo not recorded: %+v", row)
	}
	files, _ := filepath.Glob(filepath.Join(e.st.UploadDir(), row.PhotoFile+"-*.webp"))
	if len(files) != 2 {
		t.Fatalf("derivatives: %v", files)
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !(cfg.Width == 400 && cfg.Height == 500) && !(cfg.Width == 800 && cfg.Height == 1000) {
			t.Errorf("%s is %dx%d", filepath.Base(f), cfg.Width, cfg.Height)
		}
	}
	if _, err := os.Stat(filepath.Join(e.st.UploadDir(), row.PhotoFile+"-orig.jpg")); err != nil {
		t.Error("original not kept")
	}
	page := c.get("/en/coaches/gocha-butbaia").Body.String()
	if !strings.Contains(page, row.PhotoBase+"-800.webp") {
		t.Errorf("public page does not use the upload: want %s", row.PhotoBase)
	}
	if r := c.get(row.PhotoBase + "-400.webp"); r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/webp" || !strings.Contains(r.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("serving the derivative: %d %s", r.Code, r.Header().Get("Content-Type"))
	}
	if r := c.get("/uploads/" + row.PhotoFile + "-orig.jpg"); r.Code != http.StatusNotFound {
		t.Errorf("original must stay private: %d", r.Code)
	}

	// Moving the focus re-crops from the original and changes the URL.
	oldBase := row.PhotoBase
	if rec := c.post("/admin/coaches/c-gocha-butbaia/photo/focus", "focus_preset", "top"); rec.Code != http.StatusSeeOther {
		t.Fatalf("focus: %d", rec.Code)
	}
	row, _ = e.st.CoachRow("c-gocha-butbaia")
	if row.PhotoBase == oldBase || row.FocusY != 0.2 {
		t.Errorf("focus not applied: %+v", row)
	}
	if r := c.get(row.PhotoBase + "-400.webp"); r.Code != http.StatusOK {
		t.Errorf("re-cropped derivative missing: %d", r.Code)
	}
	if r := c.get(oldBase + "-400.webp"); r.Code != http.StatusNotFound {
		t.Errorf("stale derivative still served: %d", r.Code)
	}

	// Remove: the card falls back to initials.
	c.post("/admin/coaches/c-gocha-butbaia/photo/delete")
	row, _ = e.st.CoachRow("c-gocha-butbaia")
	if row.Photo || row.PhotoFile != "" {
		t.Errorf("photo not cleared: %+v", row)
	}
	if files, _ := filepath.Glob(filepath.Join(e.st.UploadDir(), "c-gocha-butbaia-*")); len(files) != 0 {
		t.Errorf("files left after delete: %v", files)
	}
}
