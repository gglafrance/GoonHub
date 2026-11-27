// Package client is the worker's HTTP client of the GoonHub worker API
// (docs/REMOTE_WORKERS.md, section 7).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"goonhub/pkg/workerproto"
)

// Client calls the server API with the worker token.
type Client struct {
	base      string
	token     string
	userAgent string
	http      *http.Client
	// CallTimeout bounds one JSON call (claim adds its long-poll wait).
	CallTimeout time.Duration
}

// New returns a client of the server at base (e.g. "http://goonhub.tailnet.ts.net:8080").
func New(base, token, userAgent string) *Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   8,
		ExpectContinueTimeout: time.Second,
	}
	return &Client{
		base:        strings.TrimRight(base, "/"),
		token:       token,
		userAgent:   userAgent,
		http:        &http.Client{Transport: tr},
		CallTimeout: 30 * time.Second,
	}
}

// HTTPClient returns the underlying client (media downloads use it too).
func (c *Client) HTTPClient() *http.Client { return c.http }

// Base returns the server base URL.
func (c *Client) Base() string { return c.base }

// ---------------------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------------------

// APIError is a non-2xx answer of the server.
type APIError struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("server answered %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("server answered %d: %s", e.Status, e.Message)
}

// AsAPIError returns the APIError in err's chain.
func AsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// IsCode reports whether err is an APIError with one of the codes.
func IsCode(err error, codes ...string) bool {
	ae, ok := AsAPIError(err)
	if !ok {
		return false
	}
	for _, c := range codes {
		if ae.Code == c {
			return true
		}
	}
	return false
}

// StatusOf returns the HTTP status of an APIError (0 otherwise).
func StatusOf(err error) int {
	if ae, ok := AsAPIError(err); ok {
		return ae.Status
	}
	return 0
}

// IsTransient reports whether a call may succeed if retried later: network errors,
// timeouts, 5xx (commit_failed included), 507 server_disk_full, 429, 408.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	ae, ok := AsAPIError(err)
	if !ok {
		return true // network error, timeout, broken body...
	}
	return ae.Status >= 500 || ae.Status == http.StatusTooManyRequests || ae.Status == http.StatusRequestTimeout
}

// IsLeaseGone reports whether the server says this lease is over for good: lease_lost,
// job_cancelled, scene_gone, job_not_found.
func IsLeaseGone(err error) bool {
	return IsCode(err, workerproto.CodeLeaseLost, workerproto.CodeJobCancelled, workerproto.CodeSceneGone, workerproto.CodeJobNotFound)
}

func readAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	ae := &APIError{Status: resp.StatusCode}
	var er workerproto.ErrorResponse
	if json.Unmarshal(body, &er) == nil && (er.Error != "" || er.Code != "") {
		ae.Code, ae.Message, ae.Details = er.Code, er.Error, er.Details
	} else {
		ae.Message = strings.TrimSpace(string(body))
		if ae.Message == "" {
			ae.Message = http.StatusText(resp.StatusCode)
		}
	}
	return ae
}

// ---------------------------------------------------------------------------------------
// Plumbing
// ---------------------------------------------------------------------------------------

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(workerproto.HeaderAuthorization, "Bearer "+c.token)
	req.Header.Set(workerproto.HeaderAPIVersion, strconv.Itoa(workerproto.APIVersion))
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

func (c *Client) doJSON(ctx context.Context, timeout time.Duration, method, path string, in, out any) error {
	if timeout <= 0 {
		timeout = c.CallTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("failed to encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return readAPIError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s %s: invalid response: %w", method, path, err)
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Endpoints
// ---------------------------------------------------------------------------------------

// Register opens a session (7.1).
func (c *Client) Register(ctx context.Context, req *workerproto.RegisterRequest) (*workerproto.RegisterResponse, error) {
	var out workerproto.RegisterResponse
	if err := c.doJSON(ctx, 0, http.MethodPost, workerproto.PathRegister, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Claim long-polls for assignments (7.2).
func (c *Client) Claim(ctx context.Context, req *workerproto.ClaimRequest) (*workerproto.ClaimResponse, error) {
	var out workerproto.ClaimResponse
	timeout := c.CallTimeout + time.Duration(req.WaitSec)*time.Second
	if err := c.doJSON(ctx, timeout, http.MethodPost, workerproto.PathClaim, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Heartbeat renews the leases (7.3).
func (c *Client) Heartbeat(ctx context.Context, req *workerproto.HeartbeatRequest) (*workerproto.HeartbeatResponse, error) {
	var out workerproto.HeartbeatResponse
	if err := c.doJSON(ctx, 0, http.MethodPost, workerproto.PathHeartbeat, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MediaURL asks a fresh signed media URL (7.4).
func (c *Client) MediaURL(ctx context.Context, jobID string, generation int64) (*workerproto.MediaURLResponse, error) {
	var out workerproto.MediaURLResponse
	err := c.doJSON(ctx, 0, http.MethodPost, workerproto.JobPath(workerproto.PathJobMediaURL, jobID),
		&workerproto.MediaURLRequest{Generation: generation}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Complete commits a job (7.6).
func (c *Client) Complete(ctx context.Context, jobID string, req *workerproto.CompleteRequest) (*workerproto.CompleteResponse, error) {
	var out workerproto.CompleteResponse
	// Commits move files and write the DB: give them more time than a plain call.
	if err := c.doJSON(ctx, 2*c.CallTimeout, http.MethodPost, workerproto.JobPath(workerproto.PathJobComplete, jobID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Fail reports a failed job (7.7).
func (c *Client) Fail(ctx context.Context, jobID string, req *workerproto.FailRequest) error {
	return c.doJSON(ctx, 0, http.MethodPost, workerproto.JobPath(workerproto.PathJobFail, jobID), req, nil)
}

// Release gives a job back (7.8).
func (c *Client) Release(ctx context.Context, jobID string, req *workerproto.ReleaseRequest) error {
	return c.doJSON(ctx, 0, http.MethodPost, workerproto.JobPath(workerproto.PathJobRelease, jobID), req, nil)
}

// ReleaseAll gives back every job of the session (7.8).
func (c *Client) ReleaseAll(ctx context.Context, req *workerproto.ReleaseAllRequest) (*workerproto.ReleaseResponse, error) {
	var out workerproto.ReleaseResponse
	if err := c.doJSON(ctx, 0, http.MethodPost, workerproto.PathReleaseAll, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadStatus returns how much of an artifact the server holds for a generation (7.5).
func (c *Client) UploadStatus(ctx context.Context, jobID, name string, generation int64) (*workerproto.UploadStatus, error) {
	var out workerproto.UploadStatus
	path := workerproto.ArtifactPath(jobID, name) + "?generation=" + strconv.FormatInt(generation, 10)
	if err := c.doJSON(ctx, 0, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutArtifact sends n bytes of body as the part of artifact ref starting at offset (7.5).
// timeout bounds the whole request.
func (c *Client) PutArtifact(ctx context.Context, jobID string, generation int64, ref workerproto.ArtifactRef,
	offset int64, body io.Reader, n int64, timeout time.Duration) (*workerproto.UploadStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodPut, workerproto.ArtifactPath(jobID, ref.Name), body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = n
	if n == 0 {
		req.Body = http.NoBody
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(workerproto.HeaderLeaseGeneration, strconv.FormatInt(generation, 10))
	req.Header.Set(workerproto.HeaderUploadOffset, strconv.FormatInt(offset, 10))
	req.Header.Set(workerproto.HeaderArtifactSize, strconv.FormatInt(ref.Size, 10))
	req.Header.Set(workerproto.HeaderArtifactSHA256, ref.SHA256)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PUT artifact %s: %w", ref.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, readAPIError(resp)
	}
	var out workerproto.UploadStatus
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("PUT artifact %s: invalid response: %w", ref.Name, err)
	}
	return &out, nil
}

// ---------------------------------------------------------------------------------------
// Media (signed URL, no bearer)
// ---------------------------------------------------------------------------------------

// MediaInfo is what a HEAD of the media URL says.
type MediaInfo struct {
	Size         int64
	ETag         string
	LastModified string
}

// MediaHead checks a signed media URL.
func (c *Client) MediaHead(ctx context.Context, mediaURL string) (*MediaInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, c.CallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD media: %w", redactErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, mediaError(resp)
	}
	return &MediaInfo{Size: resp.ContentLength, ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}, nil
}

// MediaGet starts a GET of the media URL from offset (Range when offset > 0, with
// If-Range when a validator is known). The caller closes the body.
func (c *Client) MediaGet(ctx context.Context, mediaURL string, offset int64, ifRange string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
		if ifRange != "" {
			req.Header.Set("If-Range", ifRange)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET media: %w", redactErr(err))
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		defer resp.Body.Close()
		return nil, mediaError(resp)
	}
	return resp, nil
}

// mediaError turns a media answer into an APIError. HEAD answers have no body: the code
// comes from the status (403 = url_expired, 409 = lease_lost).
func mediaError(resp *http.Response) error {
	err := readAPIError(resp)
	if ae, ok := err.(*APIError); ok && ae.Code == "" {
		switch resp.StatusCode {
		case http.StatusForbidden:
			ae.Code = workerproto.CodeURLExpired
		case http.StatusConflict:
			ae.Code = workerproto.CodeLeaseLost
		case http.StatusGone:
			ae.Code = workerproto.CodeSceneGone
		}
	}
	return err
}

// redactErr hides the media URL signature in a transport error.
func redactErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		ue.URL = RedactURL(ue.URL)
	}
	return err
}

var sigRe = regexp.MustCompile(`([?&]` + workerproto.MediaParamSignature + `=)[0-9A-Fa-f]+`)

// RedactText hides media URL signatures in free text (ffmpeg output quoting the input).
func RedactText(s string) string { return sigRe.ReplaceAllString(s, "${1}REDACTED") }

// RedactURL hides the signature of a media URL in logs.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<invalid url>"
	}
	q := u.Query()
	if q.Has(workerproto.MediaParamSignature) {
		q.Set(workerproto.MediaParamSignature, "REDACTED")
		u.RawQuery = q.Encode()
	}
	return u.String()
}
