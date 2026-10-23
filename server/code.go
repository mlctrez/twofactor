//go:build !wasm

package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mlctrez/twofactor/store"
)

// registerCodeRoutes adds read-only endpoints that expose entry labels and
// current TOTP codes computed from the storage file. Secrets are never
// returned or logged.
func registerCodeRoutes(r gin.IRoutes, storagePath string, now func() time.Time) {
	r.GET("/api/labels", labelsHandler(storagePath))
	r.GET("/api/code", codeHandler(storagePath, now))
}

func loadStorage(storagePath string) (*store.Storage, error) {
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return nil, err
	}
	s := &store.Storage{}
	if err = json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

func jsonError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func storageError(c *gin.Context, err error) {
	// log the error type only (path errors are safe to log in full) so file
	// contents can never end up in the log
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		log.Printf("unable to load storage: %v", err)
	} else {
		log.Printf("unable to load storage: %T", err)
	}
	jsonError(c, http.StatusInternalServerError, "storage unavailable")
}

func labelsHandler(storagePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		s, err := loadStorage(storagePath)
		if err != nil {
			storageError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"labels": s.Labels()})
	}
}

func codeHandler(storagePath string, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		label := c.Query("label")
		if label == "" {
			jsonError(c, http.StatusBadRequest, "label query parameter is required")
			return
		}
		s, err := loadStorage(storagePath)
		if err != nil {
			storageError(c, err)
			return
		}
		param, err := s.Find(label)
		if err != nil {
			var ambiguous *store.AmbiguousError
			switch {
			case errors.As(err, &ambiguous):
				c.JSON(http.StatusConflict, gin.H{"error": store.ErrAmbiguous.Error(), "matches": ambiguous.Matches})
			case errors.Is(err, store.ErrNotFound):
				jsonError(c, http.StatusNotFound, err.Error())
			default:
				jsonError(c, http.StatusInternalServerError, "lookup failed")
			}
			return
		}
		code, err := store.GenerateCode(param, now())
		if err != nil {
			if errors.Is(err, store.ErrNotTotp) {
				jsonError(c, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("unable to generate code: %v", err)
			jsonError(c, http.StatusInternalServerError, "unable to generate code")
			return
		}
		c.JSON(http.StatusOK, code)
	}
}
