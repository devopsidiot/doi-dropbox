// ============================================================================
// config.example.js  --  The shape of frontend/config.js
// ============================================================================
//
// This file holds the (non-secret) settings the web page needs to find your
// login system and API. None of these are passwords -- they're public
// identifiers, safe to ship in a web page.
//
// You don't hand-edit config.js in normal use: scripts/gen-config.sh reads your
// Terraform outputs and writes it for you. config.js is gitignored precisely
// because it is generated and differs per deployment; this example is the
// tracked copy that documents the shape.
//
//   ./scripts/gen-config.sh          # writes frontend/config.js
//
// "const CONFIG = { ... }" creates a single settings object the rest of the
// app reads from (e.g. CONFIG.clientId).
// ============================================================================

const CONFIG = {
  // The AWS region your Cognito user pool lives in.
  region: "us-east-1",

  // Your Cognito "app client" ID -- a public identifier for the login system.
  clientId: "REPLACE_WITH_COGNITO_CLIENT_ID",

  // The web address of your API Gateway (where the Lambda lives).
  apiBaseUrl: "https://REPLACE_WITH_API_ID.execute-api.us-east-1.amazonaws.com",

  // Optional. Overrides the Cognito endpoint the page talks to. Leave it unset
  // in a real deployment -- app.js derives the real one from `region` above.
  // The local Docker preview sets it to point at a mock; see docker/README.md.
  // cognitoUrl: "http://localhost:8080/mock-cognito/",
};
