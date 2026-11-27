package workerproto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Query parameters of the signed media URL (PathMedia).
const (
	MediaParamGeneration = "g"
	MediaParamExpires    = "exp"
	MediaParamSignature  = "sig"
)

// mediaSigningPayload is what the media URL signature covers.
func mediaSigningPayload(jobID string, generation int64, expires int64) string {
	return fmt.Sprintf("goonhub-worker-media:v1:%s:%d:%d", jobID, generation, expires)
}

// SignMedia returns the hex HMAC-SHA256 signature of a media URL.
func SignMedia(secret []byte, jobID string, generation int64, expires time.Time) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(mediaSigningPayload(jobID, generation, expires.Unix())))
	return hex.EncodeToString(mac.Sum(nil))
}

// MediaURL builds the signed media URL of a job under baseURL (the server URL the worker
// uses, e.g. "http://goonhub.tailnet.ts.net:8080").
func MediaURL(baseURL string, secret []byte, jobID string, generation int64, expires time.Time) string {
	q := url.Values{}
	q.Set(MediaParamGeneration, strconv.FormatInt(generation, 10))
	q.Set(MediaParamExpires, strconv.FormatInt(expires.Unix(), 10))
	q.Set(MediaParamSignature, SignMedia(secret, jobID, generation, expires))
	return baseURL + JobPath(PathMedia, jobID) + "?" + q.Encode()
}

// VerifyMedia checks the signature and expiry of a media URL's query parameters and returns
// the generation it was issued for. The caller must still check that this generation is the
// job's current lease generation.
func VerifyMedia(secret []byte, jobID string, query url.Values, now time.Time) (int64, error) {
	gen, err := strconv.ParseInt(query.Get(MediaParamGeneration), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid generation")
	}
	exp, err := strconv.ParseInt(query.Get(MediaParamExpires), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expiry")
	}
	want := SignMedia(secret, jobID, gen, time.Unix(exp, 0))
	got := query.Get(MediaParamSignature)
	if !hmac.Equal([]byte(want), []byte(got)) {
		return 0, fmt.Errorf("invalid signature")
	}
	if now.Unix() > exp {
		return gen, fmt.Errorf("url expired")
	}
	return gen, nil
}
