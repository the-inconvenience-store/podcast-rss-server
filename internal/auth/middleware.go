package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

var constantTimeCompare = subtle.ConstantTimeCompare

type Checker struct {
	keyHashes [][sha256.Size]byte
}

func NewChecker(keys []string) Checker {
	hashes := make([][sha256.Size]byte, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		hashes = append(hashes, sha256.Sum256([]byte(key)))
	}
	return Checker{keyHashes: hashes}
}

func (c Checker) Valid(candidate string) bool {
	candidateHash := sha256.Sum256([]byte(candidate))
	var matched int
	for _, keyHash := range c.keyHashes {
		matched |= constantTimeCompare(candidateHash[:], keyHash[:])
	}
	return matched == 1 && len(c.keyHashes) > 0
}

func Middleware(keys []string) func(http.Handler) http.Handler {
	checker := NewChecker(keys)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !checker.Valid(apiKeyFromRequest(r)) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func apiKeyFromRequest(r *http.Request) string {
	if value := r.Header.Get("Authorization"); value != "" {
		scheme, token, ok := strings.Cut(value, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(token)
		}
		return ""
	}
	return r.Header.Get("X-API-Key")
}
