package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The upload path is the interesting half of this CLI, and it is reachable in a
// test without any AWS involvement: everything after authentication is plain
// HTTP. These tests stand up a fake backend that plays both roles the real
// system plays — the API that mints a presigned URL, and the bucket that
// receives the PUT — so the request the CLI actually puts on the wire is what
// gets asserted on.
//
// authenticate() is not covered here. It reads a password straight from the
// terminal and talks to Cognito, so exercising it needs an interface seam that
// does not exist yet, and adding one touches the auth flow. See the note at the
// bottom of this file.
//
// These tests mutate package-level config (apiBaseURL and friends), so they
// must not call t.Parallel().

// recordedRequest is what the fake backend saw, so a test can assert on it
// after the fact rather than inside the handler where a failure would be
// reported on the wrong goroutine.
type recordedRequest struct {
	method        string
	path          string
	authorization string
	contentType   string
	contentLength int64
	body          string
}

type fakeBackend struct {
	server *httptest.Server

	mu       sync.Mutex
	requests []recordedRequest

	// Knobs for the unhappy paths.
	uploadURLStatus int    // status for POST /upload-url (default 200)
	uploadURLBody   string // raw body for POST /upload-url; overrides the default JSON
	putStatus       int    // status for the PUT (default 200)
}

// newFakeBackend wires up a server that answers both calls the CLI makes and
// points apiBaseURL at it for the duration of the test.
func newFakeBackend(t *testing.T) *fakeBackend {
	t.Helper()

	fb := &fakeBackend{}

	mux := http.NewServeMux()

	// The API that mints the presigned URL.
	mux.HandleFunc("/upload-url", func(w http.ResponseWriter, r *http.Request) {
		fb.record(r)

		if fb.uploadURLStatus != 0 && fb.uploadURLStatus != http.StatusOK {
			w.WriteHeader(fb.uploadURLStatus)
			_, _ = io.WriteString(w, `{"error":"nope"}`)
			return
		}

		if fb.uploadURLBody != "" {
			_, _ = io.WriteString(w, fb.uploadURLBody)
			return
		}

		// The "presigned URL" points back at this same server, which is what
		// lets a single test cover the mint-then-PUT round trip.
		_, _ = fmt.Fprintf(w, `{"uploadUrl":%q,"key":"2026-01-01_00-00-00/file"}`,
			fb.server.URL+"/presigned-put")
	})

	// Stands in for S3 receiving the presigned PUT.
	mux.HandleFunc("/presigned-put", func(w http.ResponseWriter, r *http.Request) {
		fb.record(r)

		if fb.putStatus != 0 {
			w.WriteHeader(fb.putStatus)
		}
	})

	fb.server = httptest.NewServer(mux)
	t.Cleanup(fb.server.Close)

	// Package-level config is what the code under test reads. Restore it so
	// tests stay independent of each other regardless of order.
	previous := apiBaseURL
	apiBaseURL = fb.server.URL
	t.Cleanup(func() { apiBaseURL = previous })

	return fb
}

func (fb *fakeBackend) record(r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	fb.mu.Lock()
	defer fb.mu.Unlock()

	fb.requests = append(fb.requests, recordedRequest{
		method:        r.Method,
		path:          r.URL.Path,
		authorization: r.Header.Get("Authorization"),
		contentType:   r.Header.Get("Content-Type"),
		contentLength: r.ContentLength,
		body:          string(body),
	})
}

func (fb *fakeBackend) seen() []recordedRequest {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	return append([]recordedRequest(nil), fb.requests...)
}

// writeTempFile returns the path to a file containing contents.
func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}

	return path
}

func TestRequestUploadURLReturnsTheMintedURL(t *testing.T) {
	fb := newFakeBackend(t)

	got, err := requestUploadURL(context.Background(), "token-abc", "/tmp/notes.txt")
	if err != nil {
		t.Fatalf("requestUploadURL: unexpected error: %v", err)
	}

	want := fb.server.URL + "/presigned-put"
	if got != want {
		t.Errorf("upload URL: got %q, want %q", got, want)
	}
}

func TestRequestUploadURLSendsTheBearerToken(t *testing.T) {
	fb := newFakeBackend(t)

	if _, err := requestUploadURL(context.Background(), "token-abc", "/tmp/notes.txt"); err != nil {
		t.Fatalf("requestUploadURL: unexpected error: %v", err)
	}

	seen := fb.seen()
	if len(seen) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(seen))
	}

	if got, want := seen[0].authorization, "Bearer token-abc"; got != want {
		t.Errorf("Authorization header: got %q, want %q", got, want)
	}
}

// The API is asked for a *basename*, never the caller's local path. Uploading
// /home/dan/taxes/2025.pdf should not tell the server anything about the
// directory it came from, and the Lambda's own filename validation rejects
// anything containing a separator anyway.
func TestRequestUploadURLSendsOnlyTheBasename(t *testing.T) {
	fb := newFakeBackend(t)

	if _, err := requestUploadURL(context.Background(), "t", "/home/dan/private/notes.txt"); err != nil {
		t.Fatalf("requestUploadURL: unexpected error: %v", err)
	}

	body := fb.seen()[0].body

	if !strings.Contains(body, `"filename":"notes.txt"`) {
		t.Errorf("request body should carry the basename, got: %s", body)
	}

	if strings.Contains(body, "private") || strings.Contains(body, "/home/dan") {
		t.Errorf("request body leaked the local directory: %s", body)
	}
}

func TestRequestUploadURLDerivesContentType(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		// An extension the mime package does not know falls back rather than
		// guessing, which is the behavior S3 gets told about.
		{"unknown extension falls back", "archive.zzzznotreal", "application/octet-stream"},
		{"no extension falls back", "README", "application/octet-stream"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := newFakeBackend(t)

			if _, err := requestUploadURL(context.Background(), "t", tc.filename); err != nil {
				t.Fatalf("requestUploadURL: unexpected error: %v", err)
			}

			want := fmt.Sprintf(`"contentType":%q`, tc.want)
			if body := fb.seen()[0].body; !strings.Contains(body, want) {
				t.Errorf("request body: got %s, want it to contain %s", body, want)
			}
		})
	}

	// A type the mime package does know is passed through rather than
	// flattened to the fallback. Asserting a substring keeps this from
	// breaking on the charset suffix, which varies by platform.
	t.Run("known extension is passed through", func(t *testing.T) {
		fb := newFakeBackend(t)

		if _, err := requestUploadURL(context.Background(), "t", "page.html"); err != nil {
			t.Fatalf("requestUploadURL: unexpected error: %v", err)
		}

		if body := fb.seen()[0].body; !strings.Contains(body, `"contentType":"text/html`) {
			t.Errorf("expected an html content type, got: %s", body)
		}
	})
}

func TestRequestUploadURLReportsAPIFailure(t *testing.T) {
	fb := newFakeBackend(t)
	fb.uploadURLStatus = http.StatusForbidden

	_, err := requestUploadURL(context.Background(), "expired-token", "notes.txt")
	if err == nil {
		t.Fatal("expected an error when the API rejects the request")
	}

	// The status has to survive into the message: 403 means the token expired
	// and 500 means the backend broke, and the user can only tell those apart
	// if the number is in the error.
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should name the status code, got: %v", err)
	}
}

func TestRequestUploadURLReportsUnparseableReply(t *testing.T) {
	fb := newFakeBackend(t)
	fb.uploadURLBody = "this is not json"

	if _, err := requestUploadURL(context.Background(), "t", "notes.txt"); err == nil {
		t.Fatal("expected an error when the API returns a non-JSON body")
	}
}

func TestUploadOneSendsTheFileContents(t *testing.T) {
	fb := newFakeBackend(t)
	path := writeTempFile(t, "notes.txt", "the file contents")

	if err := uploadOne(context.Background(), "t", path); err != nil {
		t.Fatalf("uploadOne: unexpected error: %v", err)
	}

	seen := fb.seen()
	if len(seen) != 2 {
		t.Fatalf("expected a mint and a PUT, got %d requests", len(seen))
	}

	put := seen[1]

	if put.method != http.MethodPut {
		t.Errorf("second request method: got %s, want PUT", put.method)
	}

	if put.body != "the file contents" {
		t.Errorf("uploaded body: got %q, want %q", put.body, "the file contents")
	}
}

// The presigned URL is minted for a specific content type; the PUT has to use
// the same one or S3 rejects the signature.
func TestUploadOnePutsWithTheSameContentType(t *testing.T) {
	fb := newFakeBackend(t)
	path := writeTempFile(t, "page.html", "<p>hi</p>")

	if err := uploadOne(context.Background(), "t", path); err != nil {
		t.Fatalf("uploadOne: unexpected error: %v", err)
	}

	seen := fb.seen()
	mintedFor := seen[0].body
	putAs := seen[1].contentType

	if !strings.Contains(mintedFor, fmt.Sprintf(`"contentType":%q`, putAs)) {
		t.Errorf("PUT content type %q does not match what the URL was minted for: %s",
			putAs, mintedFor)
	}
}

func TestUploadOneReportsAMissingFile(t *testing.T) {
	newFakeBackend(t)

	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")

	err := uploadOne(context.Background(), "t", missing)
	if err == nil {
		t.Fatal("expected an error for a file that does not exist")
	}

	// The path belongs in the message; a batch upload prints this line and the
	// user needs to know which file it was about.
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error should name the file, got: %v", err)
	}
}

func TestUploadOneReportsAFailedPut(t *testing.T) {
	fb := newFakeBackend(t)
	fb.putStatus = http.StatusInternalServerError

	path := writeTempFile(t, "notes.txt", "x")

	err := uploadOne(context.Background(), "t", path)
	if err == nil {
		t.Fatal("expected an error when the PUT is rejected")
	}

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should name the status code, got: %v", err)
	}
}

// The documented batch behavior: one bad file must not strand the rest, and the
// command must still exit non-zero. This is the property that makes the tool
// safe to put in a cron job.
func TestUploadAllContinuesPastAFailure(t *testing.T) {
	fb := newFakeBackend(t)

	first := writeTempFile(t, "first.txt", "one")
	last := writeTempFile(t, "last.txt", "three")
	missing := filepath.Join(t.TempDir(), "missing.txt")

	err := uploadAll(context.Background(), "t", []string{first, missing, last})
	if err == nil {
		t.Fatal("expected a non-nil error so the exit code reflects the failure")
	}

	var uploaded []string

	for _, r := range fb.seen() {
		if r.method == http.MethodPut {
			uploaded = append(uploaded, r.body)
		}
	}

	if len(uploaded) != 2 {
		t.Fatalf("expected the two good files to upload, got %d PUTs", len(uploaded))
	}

	// Specifically: the file *after* the failure still went up.
	if uploaded[1] != "three" {
		t.Errorf("the file after the failure did not upload; PUT bodies were %q", uploaded)
	}
}

func TestUploadAllSucceedsWhenEveryFileUploads(t *testing.T) {
	newFakeBackend(t)

	a := writeTempFile(t, "a.txt", "a")
	b := writeTempFile(t, "b.txt", "b")

	if err := uploadAll(context.Background(), "t", []string{a, b}); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// runUpload must refuse before it prompts for a password: making someone type
// their password and MFA code only to be told a flag was missing is a bad
// trade, and it is the one branch of runUpload reachable without Cognito.
func TestRunUploadRequiresConfiguration(t *testing.T) {
	cases := []struct {
		name                        string
		clientID, apiURL, uploadFor string
	}{
		{"no client id", "", "https://api.example.com", "dan"},
		{"no api url", "abc123", "", "dan"},
		{"no username", "abc123", "https://api.example.com", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previousClient, previousAPI, previousUser := clientID, apiBaseURL, username
			t.Cleanup(func() {
				clientID, apiBaseURL, username = previousClient, previousAPI, previousUser
			})

			clientID, apiBaseURL, username = tc.clientID, tc.apiURL, tc.uploadFor

			err := runUpload(uploadCmd, []string{"notes.txt"})
			if err == nil {
				t.Fatal("expected an error when configuration is incomplete")
			}

			if !strings.Contains(err.Error(), "missing settings") {
				t.Errorf("error should explain what is missing, got: %v", err)
			}
		})
	}
}
