package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type uploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
}

type uploadResponse struct {
	UploadURL string `json:"uploadUrl"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expiresIn"`
}

type downloadRequest struct {
	Key string `json:"key"`
}

type downloadResponse struct {
	DownloadURL string `json:"downloadUrl"`
	Key         string `json:"key"`
	ExpiresIn   int    `json:"expiresIn"`
}

// fileEntry is one row in the "Your files" panel.
type fileEntry struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
}

type listResponse struct {
	Files []fileEntry `json:"files"`
}

var (
	presignClient *s3.PresignClient
	s3Client      *s3.Client
	bucketName    string
	expirySeconds int
)

// maxListedFiles caps what a single list call returns. The panel is a browsing
// aid, not an inventory system, and an unbounded list would grow without limit
// as uploads accumulate.
const maxListedFiles = 200

// Spaces are permitted: they are ordinary in real filenames and harmless in an
// S3 key (the SDK percent-encodes the key when it signs the URL). The pattern is
// an allowlist, so it is still what keeps shell metacharacters, quotes and
// path separators out of the key; traversal is rejected separately above.
var filenamePattern = regexp.MustCompile(`^[A-Za-z0-9._ -]+$`)

// keyPattern matches exactly what handleUploadURL mints: a timestamp directory
// followed by a filename that already satisfies filenamePattern. Anything else
// is not one of our objects.
var keyPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}/[A-Za-z0-9._ -]+$`)

func init() {
	bucketName = os.Getenv("BUCKET_NAME")
	parsed, err := strconv.Atoi(os.Getenv("URL_EXPIRY_SECONDS"))

	if err != nil {
		parsed = 300
	}

	expirySeconds = parsed

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("could not load AWS config: %v", err)
	}

	s3Client = s3.NewFromConfig(cfg)
	presignClient = s3.NewPresignClient(s3Client)
}

func jsonResponse(statusCode int, body any) (events.APIGatewayV2HTTPResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error building response"}`,
		}, nil
	}

	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(payload),
	}, nil
}

func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename is required")
	}

	if len(name) > 255 {
		return fmt.Errorf("filename is too long (max 255 characters)")
	}

	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("filename contains an illegal path")
	}

	if !filenamePattern.MatchString(name) {
		return fmt.Errorf("filename has characters that aren't allowed")
	}

	return nil
}

// validateKey checks an object key supplied by the client before it is used to
// mint a download URL. Keys are minted by handleUploadURL in exactly one shape
// — a timestamp directory plus an already-validated filename — so anything that
// does not match that shape did not come from us.
//
// This matters more than the filename check does. A presigned GET is read
// access to whatever key it names, so an unvalidated key here would let a
// logged-in caller mint a URL for any object in the bucket. Today the bucket
// holds only uploads, but "the bucket happens to contain nothing else" is not a
// control, and it would stop being true the moment anything else is stored here.
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}

	if len(key) > 512 {
		return fmt.Errorf("key is too long")
	}

	if strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return fmt.Errorf("key contains an illegal path")
	}

	if !keyPattern.MatchString(key) {
		return fmt.Errorf("key is not in the expected form")
	}

	return nil
}

// callerName pulls the username out of the JWT the authorizer already verified,
// for logging. API Gateway rejects the request before we run if the token is
// missing or invalid, so this is never the thing granting access.
func callerName(request events.APIGatewayV2HTTPRequest) string {
	if request.RequestContext.Authorizer == nil || request.RequestContext.Authorizer.JWT == nil {
		return "unknown"
	}

	return request.RequestContext.Authorizer.JWT.Claims["username"]
}

// handleRequest routes by method and path. API Gateway sends every route to
// this one function, so the dispatch has to happen here.
func handleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	method := request.RequestContext.HTTP.Method
	path := request.RequestContext.HTTP.Path

	log.Printf("%s %s from user: %s", method, path, callerName(request))

	switch {
	case method == "POST" && path == "/upload-url":
		return handleUploadURL(ctx, request)
	case method == "GET" && path == "/files":
		return handleListFiles(ctx)
	case method == "POST" && path == "/download-url":
		return handleDownloadURL(ctx, request)
	default:
		return jsonResponse(404, map[string]string{"error": "no such endpoint"})
	}
}

func handleUploadURL(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var req uploadRequest

	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return jsonResponse(400, map[string]string{"error": "request body wasn't valid JSON"})
	}

	if err := validateFilename(req.Filename); err != nil {
		return jsonResponse(400, map[string]string{"error": err.Error()})
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	loc, _ := time.LoadLocation("America/Los_Angeles")
	datePrefix := time.Now().In(loc).Format("2006-01-02_15-04-05")

	key := fmt.Sprintf("%s/%s", datePrefix, req.Filename)

	presigned, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      &bucketName,
		Key:         &key,
		ContentType: &contentType,
	}, s3.WithPresignExpires(time.Duration(expirySeconds)*time.Second))
	if err != nil {
		log.Printf("failed to presign URL: %v", err)
		return jsonResponse(500, map[string]string{"error": "could not create upload URL"})
	}

	return jsonResponse(200, uploadResponse{
		UploadURL: presigned.URL,
		Key:       key,
		ExpiresIn: expirySeconds,
	})
}

// handleListFiles returns the objects in the bucket, newest first, so the page
// can draw the "Your files" panel.
func handleListFiles(ctx context.Context) (events.APIGatewayV2HTTPResponse, error) {
	out, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &bucketName,
		MaxKeys: aws.Int32(maxListedFiles),
	})
	if err != nil {
		log.Printf("failed to list objects: %v", err)
		return jsonResponse(500, map[string]string{"error": "could not list files"})
	}

	// Start from an empty slice rather than a nil one: encoding/json renders a
	// nil slice as null, and the page does `data.files.length` on the result.
	files := make([]fileEntry, 0, len(out.Contents))

	for _, obj := range out.Contents {
		entry := fileEntry{}

		if obj.Key != nil {
			entry.Key = *obj.Key
		}
		if obj.Size != nil {
			entry.Size = *obj.Size
		}
		if obj.LastModified != nil {
			entry.LastModified = obj.LastModified.UTC().Format(time.RFC3339)
		}

		files = append(files, entry)
	}

	// S3 returns keys in lexicographic order, and the timestamp prefix means
	// that is also chronological order. Reversing gives newest first, which is
	// what someone looking for the thing they just uploaded wants.
	slices.Reverse(files)

	return jsonResponse(200, listResponse{Files: files})
}

// handleDownloadURL mints a short-lived presigned GET for one object.
func handleDownloadURL(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var req downloadRequest

	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return jsonResponse(400, map[string]string{"error": "request body wasn't valid JSON"})
	}

	if err := validateKey(req.Key); err != nil {
		return jsonResponse(400, map[string]string{"error": err.Error()})
	}

	presigned, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucketName,
		Key:    &req.Key,
	}, s3.WithPresignExpires(time.Duration(expirySeconds)*time.Second))
	if err != nil {
		log.Printf("failed to presign download URL: %v", err)
		return jsonResponse(500, map[string]string{"error": "could not create download URL"})
	}

	return jsonResponse(200, downloadResponse{
		DownloadURL: presigned.URL,
		Key:         req.Key,
		ExpiresIn:   expirySeconds,
	})
}

func main() {
	lambda.Start(handleRequest)
}
