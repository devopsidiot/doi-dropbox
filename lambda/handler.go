package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type uploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
}

type uploadRepsonse struct {
	UploadURL string `json:"uploadUrl"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expiresIn"`
}

var (
	presignClient *s3.presignClient
	bucketName    string
	expirySeconds int
)

var filenamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

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

	s3Client := s3.NewFromConfig(cfg)
	presignClient = s3.NewPresignClient(s3Client)
}

func jsonResponse(statusCode int, body any) (events.ApiGatewayV2HTTPResponse, error) {
	payload, err := json.Marchal(body)
	if err != nil {
		return events.ApiGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error building response}`,
		}, nil
	}

	return events.ApiGatewayV2HTTPResponse{
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

func handleRequest(ctx context.Context, request events.ApiGatewayV2HTTPResponse) (events.ApiGatewayV2HTTPResponse, error) {

	if request.RequestContext.Authorizer != nill && request.RequestContext.Authorizer.JWT != nil {
		username := request.RequestContext.Authorizer.JWT.Claims["username"]

		log.Printf("upload request from user: %s", username)
	}

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

	return jsonResponse(200, uploadRepsonse{
		UploadURL: presigned.URL,
		Key:       key,
		ExpiresIn: expirySeconds,
	})
}

func main() {
	lambda.Start(handleRequest)
}
