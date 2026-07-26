# Local preview

Runs the web page in a container with stand-ins for Cognito, the API and S3, so
you can click through the whole thing without an AWS account or a deployment.

```bash
make preview      # builds the image and runs it
```

Then open <http://localhost:8080> and log in with **any** username, password and
six-digit code. Upload something, watch it appear under "Your files", download it
back.

`Ctrl-C` stops it. `make preview-stop` cleans up if a container is left behind.

## What is fake

Everything except the frontend itself:

| Real thing | What the preview does |
| --- | --- |
| Cognito | Accepts any credentials. Always issues the MFA challenge, then always succeeds. |
| API Gateway + JWT authorizer | Absent. The bearer token is never checked. |
| Lambda | Reimplemented in `preview/main.go`, matching the real validation rules. |
| S3 | An in-memory map. Everything is lost when the container stops. |
| Presigned URLs | Plain unsigned URLs pointing back at the preview server. |

**This is a development aid. Do not deploy it.** It has no authentication at all
— the login screen is theatre, and any request is served.

## What is real

The parts the frontend actually depends on, so what you see is what you would
get:

- the key format: a `2006-01-02_15-04-05/` timestamp directory plus the filename
- the filename allowlist, `[A-Za-z0-9._ -]`, rejected with the same messages
- the JSON shape of every response
- the request the browser makes, including the two-step login

That last point is the useful one. Drop a file called `résumé (final).pdf` in and
you will see it stored as `resume -final.pdf`, with the page telling you it was
renamed — that is the browser-side sanitizer in `frontend/app.js` doing its job,
and it is the behavior that keeps ordinary filenames from coming back as a bare
400 from the real API. A name with nothing ASCII in it at all, like `日本語.txt`,
becomes `upload.txt`: the extension is preserved even when the name is not.

## Where the real implementation lives

`lambda/handler.go`. Where the two disagree, that one is right — the preview is a
convenience, not a specification. If you change the validation rules, change them
there first.
