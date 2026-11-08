package localstorage

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/abysstract/abysstract-admin/backend/storage"
)

// ServeHTTP serves objects under MountPath and accepts signed uploads.
//
// GET and HEAD serve files without directory listings. Responses stop
// browsers from sniffing content types and from running uploaded HTML or SVG
// as pages of the application origin. Anyone with the URL can download a
// file.
//
// PUT accepts a body only with a valid, unexpired signature from PresignPut,
// the signed Content-Type and exactly the signed length.
func (s *Storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, ok := strings.CutPrefix(r.URL.Path, s.mountPath)
	if !ok || storage.ValidateKey(key) != nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		s.serveFile(w, r, key)
	case http.MethodPut:
		s.serveUpload(w, r, key)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (s *Storage) serveFile(w http.ResponseWriter, r *http.Request, key string) {
	f, err := s.root.Open(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	// The CSP sandbox stops Chrome's built-in viewer from rendering PDFs,
	// so PDFs rely on nosniff alone.
	if !strings.EqualFold(path.Ext(info.Name()), ".pdf") {
		h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (s *Storage) serveUpload(w http.ResponseWriter, r *http.Request, key string) {
	up, err := s.verify(key, r)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, ErrContentLength) || errors.Is(err, errMissingParameter) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, ErrNoSecret) {
			status = http.StatusNotImplemented
		}
		writeError(w, status, err)
		return
	}
	body := http.MaxBytesReader(w, r.Body, up.size)
	if err := s.Put(r.Context(), key, body, up.size, up.contentType); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge), errors.Is(err, storage.ErrSizeMismatch):
			writeError(w, http.StatusBadRequest, ErrContentLength)
		default:
			writeError(w, http.StatusInternalServerError, errors.New("localstorage: upload failed"))
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": err.Error()})
}
