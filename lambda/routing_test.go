package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// The handlers validate their input before they touch S3, so everything below
// runs with the package-level AWS clients still nil. That is deliberate: it
// keeps the tests on the part that decides what is allowed, which is the part
// worth testing, and it means the suite needs no credentials.
//
// The S3 calls themselves are not covered here — exercising those needs an
// interface seam that does not exist yet.

// request builds a minimal API Gateway v2 event for a route.
func request(method, path, body string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Body: body,
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: method,
				Path:   path,
			},
		},
	}
}

// errorFrom pulls the "error" field out of a handler's JSON body.
func errorFrom(t *testing.T, body string) string {
	t.Helper()

	var parsed struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("response body was not JSON: %q", body)
	}

	return parsed.Error
}

func TestValidateKey(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		wantOK bool
	}{
		// The one shape handleUploadURL mints.
		{"a minted key", "2026-07-26_14-30-00/notes.txt", true},
		{"spaces in the filename", "2026-07-26_14-30-00/my report v2.pdf", true},

		{"empty is rejected", "", false},
		{"a bare filename is rejected", "notes.txt", false},
		{"a different prefix is rejected", "uploads/notes.txt", false},
		{"a malformed timestamp is rejected", "2026-7-26_14-30-00/notes.txt", false},
		{"a nested path is rejected", "2026-07-26_14-30-00/sub/notes.txt", false},
		{"traversal is rejected", "2026-07-26_14-30-00/../../etc/passwd", false},
		{"a leading slash is rejected", "/2026-07-26_14-30-00/notes.txt", false},
		{"shell characters are rejected", "2026-07-26_14-30-00/x;rm -rf ~", false},

		// A presigned GET is read access to whatever key it names, so a key
		// pointing at something we never wrote must not be signable.
		{"another bucket prefix is rejected", "../other-bucket/secret.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateKey(tc.input)
			gotOK := err == nil

			if gotOK != tc.wantOK {
				t.Errorf("validateKey(%q): got allowed=%v, wanted allowed=%v (error was: %v)",
					tc.input, gotOK, tc.wantOK, err)
			}
		})
	}
}

// Every route reaches this Lambda through one handler, so an unrouted path has
// to be turned away here rather than by API Gateway.
func TestUnknownRoutesAre404(t *testing.T) {
	cases := []struct {
		name           string
		method, path   string
		wantStatusCode int
	}{
		{"unknown path", "GET", "/nope", 404},
		{"wrong method on upload-url", "GET", "/upload-url", 404},
		{"wrong method on files", "POST", "/files", 404},
		{"wrong method on download-url", "GET", "/download-url", 404},
		{"the bare root", "GET", "/", 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handleRequest(context.Background(), request(tc.method, tc.path, ""))
			if err != nil {
				t.Fatalf("handleRequest returned an error: %v", err)
			}

			if resp.StatusCode != tc.wantStatusCode {
				t.Errorf("%s %s: got status %d, want %d",
					tc.method, tc.path, resp.StatusCode, tc.wantStatusCode)
			}
		})
	}
}

func TestUploadURLRejectsBadInput(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantInError string
	}{
		{"not JSON", "this is not json", "JSON"},
		{"no filename", `{"contentType":"text/plain"}`, "required"},
		{"traversal in the filename", `{"filename":"../../etc/passwd"}`, "illegal path"},
		{"shell characters", `{"filename":"x;rm -rf ~"}`, "aren't allowed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handleRequest(context.Background(),
				request("POST", "/upload-url", tc.body))
			if err != nil {
				t.Fatalf("handleRequest returned an error: %v", err)
			}

			if resp.StatusCode != 400 {
				t.Fatalf("got status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
			}

			if msg := errorFrom(t, resp.Body); !strings.Contains(msg, tc.wantInError) {
				t.Errorf("error message %q should mention %q", msg, tc.wantInError)
			}
		})
	}
}

func TestDownloadURLRejectsBadInput(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantInError string
	}{
		{"not JSON", "this is not json", "JSON"},
		{"no key", `{}`, "required"},
		{"traversal", `{"key":"../../etc/passwd"}`, "illegal path"},
		{"an unminted shape", `{"key":"notes.txt"}`, "expected form"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := handleRequest(context.Background(),
				request("POST", "/download-url", tc.body))
			if err != nil {
				t.Fatalf("handleRequest returned an error: %v", err)
			}

			if resp.StatusCode != 400 {
				t.Fatalf("got status %d, want 400 (body: %s)", resp.StatusCode, resp.Body)
			}

			if msg := errorFrom(t, resp.Body); !strings.Contains(msg, tc.wantInError) {
				t.Errorf("error message %q should mention %q", msg, tc.wantInError)
			}
		})
	}
}

// The page does `data.files.length` on the reply, which throws if files came
// back as null. encoding/json renders a nil slice as null, so the empty case
// has to be an allocated slice.
func TestListResponseEncodesEmptyAsArray(t *testing.T) {
	payload, err := json.Marshal(listResponse{Files: make([]fileEntry, 0)})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}

	if got, want := string(payload), `{"files":[]}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// callerName reads a claim out of a token API Gateway has already verified. It
// must not panic when the authorizer context is absent, which is what a local
// invocation or a misconfigured route looks like.
func TestCallerNameHandlesAMissingAuthorizer(t *testing.T) {
	if got := callerName(request("GET", "/files", "")); got != "unknown" {
		t.Errorf("got %q, want %q", got, "unknown")
	}

	withJWT := request("GET", "/files", "")
	withJWT.RequestContext.Authorizer = &events.APIGatewayV2HTTPRequestContextAuthorizerDescription{
		JWT: &events.APIGatewayV2HTTPRequestContextAuthorizerJWTDescription{
			Claims: map[string]string{"username": "dan"},
		},
	}

	if got := callerName(withJWT); got != "dan" {
		t.Errorf("got %q, want %q", got, "dan")
	}
}
