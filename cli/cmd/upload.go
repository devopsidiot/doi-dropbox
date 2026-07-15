package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"

	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <file> [more files...]",
	Short: "Upload one or more files to your dropbox",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runUpload,
}

func authenticate(ctx context.Context) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("loading AWS config: %w", err)
	}

	client := cip.NewFromConfig(cfg)

	fmt.Print("Password: ")

	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))

	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading password %w", err)
	}

	password := string(passwordBytes)

	initResp, err := client.InitiateAuth(ctx, &cip.InitiateAuthInput{
		ClientId: aws.String(clientID),
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME": username,
			"PASSWORD": password,
		},
	})
	if err != nil {
		return "", fmt.Errorf("login failed: %w", err)
	}

	if initResp.ChallengeName == types.ChallengeNameTypeSoftwareTokenMfa {

		fmt.Print("MFA code: ")
		reader := bufio.NewReader(os.Stdin)
		code, _ := reader.ReadString('\n')
		code = strings.TrimSpace(code)

		challengeResp, err := client.RespondToAuthChallenge(ctx, &cip.RespondToAuthChallengeInput{
			ClientId:      aws.String(clientID),
			ChallengeName: types.ChallengeNameTypeSoftwareTokenMfa,
			Session:       initResp.Session,
			ChallengeResponses: map[string]string{
				"USERNAME":                username,
				"SOFTWARE_TOKEN_MFA_CODE": code,
			},
		})
		if err != nil {
			return "", fmt.Errorf("MFA check failed: %w", err)
		}

		return *challengeResp.AuthenticationResult.IdToken, nil
	}

	if initResp.AuthenticationResult != nil && initResp.AuthenticationResult.IdToken != nil {
		return *initResp.AuthenticationResult.IdToken, nil
	}

	return "", fmt.Errorf("login did not complete as expected")
}

func requestUploadURL(ctx context.Context, idToken, path string) (string, error) {

	filename := filepath.Base(path)

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	bodyBytes, _ := json.Marshal(struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
	}{Filename: filename, ContentType: contentType})

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		apiBaseURL+"/upload-url", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+idToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("calling API: %w", err)
	}

	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed struct {
		UploadURL string `json:"uploadUrl"`
		Key       string `json:"key"`
	}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("reading API reply: %w", err)
	}

	return parsed.UploadURL, nil
}

func uploadOne(ctx context.Context, idToken, path string) error {

	uploadURL, err := requestUploadURL(ctx, idToken, path)
	if err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer file.Close()

	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	putReq, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return fmt.Errorf("building upload for %s: %w", path, err)
	}
	putReq.Header.Set("Content-Type", contentType)

	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("uploading %s: %w", path, err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		return fmt.Errorf("upload of %s failed with status %d", path, putResp.StatusCode)
	}

	fmt.Printf(" uploaded: %s\n", path)
	return nil
}

func runUpload(cmd *cobra.Command, args []string) error {

	if clientID == "" || apiBaseURL == "" || username == "" {
		return fmt.Errorf("missing settings: make sure --client-id, --api-url, and --username are set (via flags or environment variables)")
	}

	ctx := cmd.Context()

	fmt.Printf("Logging in as %s...\n", username)
	idToken, err := authenticate(ctx)
	if err != nil {
		return err
	}

	anyFailed := false

	for _, path := range args {
		if err := uploadOne(ctx, idToken, path); err != nil {
			fmt.Printf("  FAILED: %v\n", err)
			anyFailed = true
		}
	}

	if anyFailed {
		return fmt.Errorf("one or more uploads failed")
	}
	fmt.Println("All uploads finished.")
	return nil
}
