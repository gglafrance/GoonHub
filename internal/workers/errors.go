// Package workers is the server side of the remote worker protocol (docs/REMOTE_WORKERS.md):
// worker registration and tokens, leases with fencing generations, the claim (pull) and the
// push dispatcher, artifact uploads into a staging area, the verified commit of a complete,
// the lease expiry sweep, the staging janitor and the per-phase routing between the local
// pools and the remote workers.
package workers

import (
	"fmt"
	"net/http"

	"goonhub/pkg/workerproto"
)

// Error is a protocol error: an HTTP status and a workerproto.Code* code.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *Error) Error() string { return e.Message }

func newError(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func errInvalid(format string, args ...any) *Error {
	return newError(http.StatusBadRequest, workerproto.CodeInvalidRequest, format, args...)
}

var (
	errUnknownSession = newError(http.StatusConflict, workerproto.CodeUnknownSession, "unknown session, register again")
	errLeaseLost      = newError(http.StatusConflict, workerproto.CodeLeaseLost, "lease lost")
	errJobCancelled   = newError(http.StatusConflict, workerproto.CodeJobCancelled, "job cancelled")
	errJobNotFound    = newError(http.StatusNotFound, workerproto.CodeJobNotFound, "job not found")
	errSceneGone      = newError(http.StatusGone, workerproto.CodeSceneGone, "scene deleted")
)

// AsError returns the protocol error of err (a 500 internal error when err is not one).
func AsError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	}
	return &Error{Status: http.StatusInternalServerError, Code: "internal_error", Message: err.Error()}
}
