package distributed

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const authHeader = "Authorization"

// SignJob creates an HMAC-SHA256 signature for a job ID using the shared secret.
func SignJob(jobID, secret string) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(jobID))
	return "Bearer " + hex.EncodeToString(mac.Sum(nil))
}

// VerifyJob checks the Authorization header against the expected HMAC for the job ID.
func VerifyJob(r *http.Request, jobID, secret string) bool {
	if secret == "" {
		return true
	}
	expected := SignJob(jobID, secret)
	auth := r.Header.Get(authHeader)
	return hmac.Equal([]byte(auth), []byte(expected))
}

// AddAuthHeader adds the HMAC signature header to an HTTP request.
func AddAuthHeader(req *http.Request, jobID, secret string) {
	if secret == "" {
		return
	}
	req.Header.Set(authHeader, SignJob(jobID, secret))
}

// AuthMiddleware returns an HTTP middleware that validates HMAC signatures.
// Health checks are always allowed without auth.
func AuthMiddleware(secret string, next http.Handler) http.Handler {
	if secret == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /health is always public; /run verifies auth in its own handler after parsing the body for jobID.
		if r.URL.Path == "/health" || r.URL.Path == "/run" {
			next.ServeHTTP(w, r)
			return
		}
		jobID := extractJobID(r)
		if jobID == "" || !VerifyJob(r, jobID, secret) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractJobID(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/cancel/") {
		return strings.TrimPrefix(r.URL.Path, "/cancel/")
	}
	if strings.HasPrefix(r.URL.Path, "/status/") {
		return strings.TrimPrefix(r.URL.Path, "/status/")
	}
	return ""
}
