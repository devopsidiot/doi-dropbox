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

		challengeResp, err := client.RespondToAuthChallenge(ctx, &cip.RespondToAuthChallenge{
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
