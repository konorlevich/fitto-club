package admin

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gen2brain/webp"
	"github.com/konorlevich/fitto-club/internal/store"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Photo pipeline (checklist §13, uploads): sniff the real type, cap the
// size, decode, apply the EXIF orientation, drop everything else EXIF had,
// crop 4:5 around the focus point and write WebP at the two card widths.
// The original (re-encoded, metadata-free, upright) is kept privately so a
// later focus change re-crops without a re-upload.

const (
	maxUpload   = 12 << 20
	webpQuality = 82
)

var photoWidths = []int{400, 800}

var errType = errors.New("unsupported type")

type decoded struct {
	img  image.Image
	hash string
}

// decodePhoto validates and decodes an upload. HEIC is recognised so the
// message can say what to do; anything else unknown is refused the same way.
func decodePhoto(b []byte) (decoded, error) {
	if len(b) > 12 && string(b[4:8]) == "ftyp" && (bytes.Contains(b[8:12], []byte("hei")) || bytes.Contains(b[8:12], []byte("mif"))) {
		return decoded{}, fmt.Errorf("%w: heic", errType)
	}
	switch ct := http.DetectContentType(b); ct {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return decoded{}, fmt.Errorf("%w: %s", errType, ct)
	}
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return decoded{}, err
	}
	if format == "jpeg" {
		img = applyOrientation(img, exifOrientation(b))
	}
	sum := sha256.Sum256(b)
	return decoded{img: img, hash: hex.EncodeToString(sum[:4])}, nil
}

// exifOrientation reads tag 0x0112 from a JPEG's APP1 segment; 1 when absent.
func exifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) && b[i] == 0xFF {
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end
			break
		}
		size := int(binary.BigEndian.Uint16(b[i+2:]))
		if size < 2 || i+2+size > len(b) {
			break
		}
		seg := b[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			t := seg[6:]
			var bo binary.ByteOrder
			switch string(t[:2]) {
			case "II":
				bo = binary.LittleEndian
			case "MM":
				bo = binary.BigEndian
			default:
				return 1
			}
			off := int(bo.Uint32(t[4:8]))
			if off+2 > len(t) {
				return 1
			}
			n := int(bo.Uint16(t[off:]))
			for k := 0; k < n; k++ {
				e := off + 2 + k*12
				if e+12 > len(t) {
					return 1
				}
				if bo.Uint16(t[e:]) == 0x0112 {
					v := int(bo.Uint16(t[e+8:]))
					if v >= 1 && v <= 8 {
						return v
					}
					return 1
				}
			}
			return 1
		}
		i += 2 + size
	}
	return 1
}

// applyOrientation rotates/flips so the pixels are upright (EXIF 1..8).
func applyOrientation(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *image.NRGBA
	if o >= 5 {
		dst = image.NewNRGBA(image.Rect(0, 0, h, w))
	} else {
		dst = image.NewNRGBA(image.Rect(0, 0, w, h))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.At(b.Min.X+x, b.Min.Y+y)
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, c)
		}
	}
	return dst
}

// cropFocus returns the largest 4:5 rectangle of src that keeps the focus
// point as central as the edges allow.
func cropFocus(src image.Rectangle, fx, fy float64) image.Rectangle {
	w, h := src.Dx(), src.Dy()
	cw, ch := w, h
	if float64(w)/float64(h) > 0.8 {
		cw = int(float64(h) * 0.8)
	} else {
		ch = int(float64(w) / 0.8)
	}
	x := int(fx*float64(w)) - cw/2
	y := int(fy*float64(h)) - ch/2
	x = max(0, min(x, w-cw))
	y = max(0, min(y, h-ch))
	return image.Rect(src.Min.X+x, src.Min.Y+y, src.Min.X+x+cw, src.Min.Y+y+ch)
}

// derive writes <base>-400.webp and <base>-800.webp from img.
func derive(dir, base string, img image.Image, fx, fy float64) error {
	crop := cropFocus(img.Bounds(), fx, fy)
	for _, w := range photoWidths {
		h := w * 5 / 4
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
		var buf bytes.Buffer
		if err := webp.Encode(&buf, dst, webp.Options{Quality: webpQuality}); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d.webp", base, w)), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// removeDerivatives deletes every file of a coach's older photo.
func removeDerivatives(dir, coachID, keepFile string) {
	files, _ := filepath.Glob(filepath.Join(dir, coachID+"-*"))
	for _, f := range files {
		if keepFile != "" && strings.HasPrefix(filepath.Base(f), keepFile) {
			continue
		}
		_ = os.Remove(f)
	}
}

func (s *Server) photoUpload(c *ctx) {
	id := c.r.PathValue("id")
	row, err := s.Store.CoachRow(id)
	if err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	p := s.newPage(c, "coaches", p_title(row))
	c.r.Body = http.MaxBytesReader(c.w, c.r.Body, maxUpload+64<<10)
	if err := c.r.ParseMultipartForm(maxUpload); err != nil {
		s.flash(c, "err", p.T("coach.photo_error_size"), "", "")
		s.redirect(c, "/admin/coaches/"+id)
		return
	}
	if c.r.FormValue("_csrf") != c.csrf {
		s.errorPage(c, http.StatusForbidden)
		return
	}
	f, _, err := c.r.FormFile("photo")
	if err != nil {
		s.flash(c, "err", p.T("coach.photo_error_decode"), "", "")
		s.redirect(c, "/admin/coaches/"+id)
		return
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil || len(b) > maxUpload {
		s.flash(c, "err", p.T("coach.photo_error_size"), "", "")
		s.redirect(c, "/admin/coaches/"+id)
		return
	}
	dec, err := decodePhoto(b)
	if err != nil {
		key := "coach.photo_error_decode"
		if errors.Is(err, errType) {
			key = "coach.photo_error_type"
		}
		s.flash(c, "err", p.T(key), "", "")
		s.redirect(c, "/admin/coaches/"+id)
		return
	}
	dir := s.Store.UploadDir()
	file := id + "-" + dec.hash
	fx, fy := 0.5, 0.4 // faces sit above the middle of a portrait
	// The private original: upright, JPEG, no metadata.
	var orig bytes.Buffer
	if err := jpeg.Encode(&orig, dec.img, &jpeg.Options{Quality: 90}); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, file+"-orig.jpg"), orig.Bytes(), 0o600); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	base := strings.TrimPrefix(store.PhotoBase(file, fx, fy), "/uploads/")
	if err := derive(dir, base, dec.img, fx, fy); err != nil {
		s.Log.WithError(err).Error("photo derivatives")
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if err := s.Store.SetCoachPhoto(id, file, fx, fy, c.user.Name, "photo"); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	removeDerivatives(dir, id, file)
	s.flash(c, "ok", p.T("coach.photo_saved"), "", "")
	s.redirect(c, "/admin/coaches/"+id+"#photo")
}

func p_title(r store.CoachRow) string { return r.Name.Get("en") }

func (s *Server) photoFocus(c *ctx) {
	id := c.r.PathValue("id")
	row, err := s.Store.CoachRow(id)
	if err != nil || row.PhotoFile == "" {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	fx, errx := strconv.ParseFloat(c.r.FormValue("focus_x"), 64)
	fy, erry := strconv.ParseFloat(c.r.FormValue("focus_y"), 64)
	switch c.r.FormValue("focus_preset") {
	case "top":
		fx, fy, errx, erry = 0.5, 0.2, nil, nil
	case "center":
		fx, fy, errx, erry = 0.5, 0.5, nil, nil
	case "bottom":
		fx, fy, errx, erry = 0.5, 0.8, nil, nil
	}
	if errx != nil || erry != nil || fx < 0 || fx > 1 || fy < 0 || fy > 1 {
		s.errorPage(c, http.StatusUnprocessableEntity)
		return
	}
	dir := s.Store.UploadDir()
	ob, err := os.ReadFile(filepath.Join(dir, row.PhotoFile+"-orig.jpg"))
	if err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	img, _, err := image.Decode(bytes.NewReader(ob))
	if err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	base := strings.TrimPrefix(store.PhotoBase(row.PhotoFile, fx, fy), "/uploads/")
	if err := derive(dir, base, img, fx, fy); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	if err := s.Store.SetCoachPhoto(id, row.PhotoFile, fx, fy, c.user.Name, "focus"); err != nil {
		s.errorPage(c, http.StatusInternalServerError)
		return
	}
	// Older crops of the same original are no longer referenced.
	files, _ := filepath.Glob(filepath.Join(dir, row.PhotoFile+"-*.webp"))
	for _, f := range files {
		if !strings.HasPrefix(filepath.Base(f), base) {
			_ = os.Remove(f)
		}
	}
	s.flash(c, "ok", c.copy(s).T("coach.photo_saved"), "", "")
	s.redirect(c, "/admin/coaches/"+id+"#photo")
}

func (s *Server) photoDelete(c *ctx) {
	id := c.r.PathValue("id")
	if err := s.Store.ClearCoachPhoto(id, c.user.Name); err != nil {
		s.errorPage(c, http.StatusNotFound)
		return
	}
	removeDerivatives(s.Store.UploadDir(), id, "")
	s.flash(c, "ok", c.copy(s).T("flash.deleted"), "", "")
	s.redirect(c, "/admin/coaches/"+id+"#photo")
}
