# devopsidiot dropbox

A private, single-user, password-plus-MFA protected place to drop files into an S3 bucket from a webpage or CLI. No AWS keys live in the browser or on remote workstations.

## How it works

Log in with a username, password, and 6 digit code from your phone. Once logged in, when you drop a file, the website essentially asks "can I upload this" and gets back a one time, limited time web address to upload straight to s3. There is no AWS password stored where you upload from.

```
You (browser or CLI)
   │  1. log in (username + password + phone code)
   ▼
Cognito  ── gives back a short-lived "you're logged in" token
   │
   │  2. "may I upload the-thing.jpg?"  (token attached)
   ▼
API Gateway ── checks the token is real, then calls ──►  Lambda function
   │                                                         │
   │           3. one-time upload URL  ◄─────────────────────┘
   ▼
You upload the file DIRECTLY to S3 using that URL
```

---

## Repo contents
```
terraform/      buckets, login system, function, website hosting
lambda/         Golang function that supplies upload tokens
cli/            Golang CLI tool to upload files without the website
frontend/       Webpage (index.html, app.js, generated config.js)
scripts/        helper scripts for setup steps that can't be automated
Makefile
```

---

## Project outline and steps

### 1. Build Lambda

Lambda is Golang that has to be compiled before terraform can package it.
Compiling and testing is built in
```
make build-lambda
make test
```

---

### 2. Create infra

Run terraform

---

### 3. Create user

Terraform doesn't create the singular account (so the password never lands in tf's files)

### 4. Turn on phone's MFA

```
./scripts/mfa.sh
```

Script logs in, shows secret to type into auth app, confirms code. After this, logins require phone auth.

---

### 5. Website go live

Generate website confi from terraform outputs, then upload it

```
make gen-config
make deploy-frontend BUCKET=<frontend bucket name> DIST=<cloudfront distribution id>
```

---

### 6. Set up the command-line uploader

Build and input your settings (all non-secret), then upload:

```
cd cli
go build -o dropbox-cli .

export COGNITO_REGION=us-east-1
export COGNITO_CLIENT_ID=<client id from terraform output>
export API_BASE_URL=<api url from terraform output>
export DROPBOX_USERNAME=<your username>

./dropbox-cli upload ~/Pictures/the_thing.jpg ~/Documents/report.pdf
```

The function will ask for your password and a phone code, then upload each file.

## Using it from another computer

- **CLI:** copy the built binary over (or rebuild with `go build`), set the four
  non-secret environment variables above, and run it. There are no AWS keys or
  credential files to copy. Every run asks for your password and phone code
  fresh — nothing is saved between runs, on any machine, by design.

---

## What each piece costs

Roughly **$0–3/month** for light personal use, dominated entirely by how much
you store in S3 (about $0.023 per GB per month). The login system, function,
API, and website hosting all sit inside AWS's free allowances at this scale.
