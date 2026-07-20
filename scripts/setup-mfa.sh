#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../terraform"
CLIENT_ID=$(terraform output -raw cognito_client_id)
cd ..

read -p "Username: " USERNAME
read -s -p "Password: " PASSWORD
echo

echo "Logging in..."
AUTH=$(aws cognito-idp initiate-auth \
  --client-id "$CLIENT_ID" \
  --auth-flow USER_PASSWORD_AUTH \
  --auth-parameters "USERNAME=$USERNAME,PASSWORD=$PASSWORD" \
  --output json)

ACCESS_TOKEN=$(echo "$AUTH" | jq -r '.AuthenticationResult.AccessToken')

if [ "$ACCESS_TOKEN" = "null" ] || [ -z "$ACCESS_TOKEN" ]; then
  echo "Could not get an access token. If this is your first login, make sure"
  echo "you set a PERMANENT password (see the admin-set-user-password step)."
  exit 1
fi

echo "Requesting an MFA secret..."
ASSOC=$(aws cognito-idp associate-software-token \
  --access-token "$ACCESS_TOKEN" \
  --output json)

SECRET=$(echo "$ASSOC" | jq -r '.SecretCode')

echo
echo "==================================================================="
echo "Type this secret into your authenticator app (choose 'enter a"
echo "setup key' / 'manual entry'):"
echo
echo "    $SECRET"
echo
echo "Your app will then show a 6-digit code that changes every 30 seconds."
echo "==================================================================="
echo

read -p "Enter the 6-digit code your app is showing now: " CODE

aws cognito-idp verify-software-token \
  --access-token "$ACCESS_TOKEN" \
  --user-code "$CODE" \
  --output json >/dev/null

echo
echo "Success! MFA is now set up. From now on, logging in requires a code"
echo "from your authenticator app."
