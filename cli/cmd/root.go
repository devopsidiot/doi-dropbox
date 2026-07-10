package cmd

import (
	"os"
	"github.com/spf13/cobra"
)

var (
	region		string
	clientID	string
	apiBaseURL	string
	username	string
)

var rootCmd = &cobra.Command{
	Use:	"dropbox-cli",
	Short:	"Upload files to your personal s3 dropbox",
	Long:	"dropbox-cli logs into your Cognito account (with MFA) and uploads\n" +
			"files to your private s3 bucket via presigned URLs. No AWS keys needed.",
}

func Execute() error {
	return rootCmd.Execute()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func init() {
	rootCmd.PersistentFlags().StringVar(&region, "region",
		envOr("COGNITO_REGION", "us-east-1"),
		"AWS region your Cognito user pool lives in")

	rootCmd.PersistentFlags().StringVar(&clientID, "client-id",
		envOr("COGNITO_CLIENT_ID", ""),
		"Cognito app client (not secret)")

	rootCmd.PersistentFlags().StringVar(&apiBaseURL, "api-url",
		envOr("API_BASE_URL", ""),
		"Base URL of your API Gateway, i.e. https://abc123.execute-api.us-east-1.amazonaws.com")

	rootCmd.PersistentFlags().StringVar(&username, "username",
		envOr("DROPBOX_USERNAME", ""),
		"Your Cognito username")

	rootCmd.AddCommand(uploadCmd)
}