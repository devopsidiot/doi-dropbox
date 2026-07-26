// ============================================================================
// app.js  --  The behavior of the web page: log in, then upload files
// ============================================================================
//
// WHAT IS THIS?
// index.html describes WHAT the page looks like. This file describes what
// HAPPENS when you use it: what the buttons do, how login works, how files get
// uploaded. It's written in JavaScript, the language every web browser runs.
//
// A FEW JAVASCRIPT BASICS:
//   - "const x = 5"  makes a value named x that won't change.
//   - "let x = 5"    makes a value that CAN change later.
//   - "function name(args) { ... }" defines a reusable block of steps.
//   - "async" / "await": some things (like talking to a server) take time.
//     "await" means "pause here until this finishes, then continue." A function
//     that uses await must be marked "async".
//   - document.getElementById("foo") finds the HTML element whose id is "foo".
//   - element.addEventListener("click", fn) means "run fn whenever this is
//     clicked." Same idea for "drop", "change", etc.
//   - fetch(url, options) sends a web request and returns a "promise" of the
//     response -- we await it.
//
// HOW LOGIN WORKS HERE:
// Cognito (the AWS login service) has a plain web API. We don't need any big
// library -- we just send it JSON with fetch(). There are two steps:
//   1) InitiateAuth       -- send username + password
//   2) RespondToAuthChallenge -- send the MFA code
// Then Cognito hands back an "ID token": a temporary string that proves you're
// logged in. We attach that token to our upload requests.
//
// Note that the token lives only in a variable on this page. Reloading loses
// it and you log in again, which is deliberate -- see docs/adr/0002.
// ============================================================================

// The web address of Cognito's API in your region, built from config.js.
// CONFIG.cognitoUrl overrides it, which is what the local Docker preview uses
// to point the page at a mock instead of real AWS. It is unset in production.
const COGNITO_URL = CONFIG.cognitoUrl || `https://cognito-idp.${CONFIG.region}.amazonaws.com/`;

// We keep a little bit of memory while the page is open:
let idToken = null;      // the proof-of-login string, once we have it
let mfaSession = null;   // a value Cognito gives us between the two login steps
let pendingUsername = ""; // remember the username between the two login steps

// --- Grab references to the HTML elements we'll interact with. ---
// Doing this once up top keeps the code below tidy.
const loginSection  = document.getElementById("login-section");
const mfaSection    = document.getElementById("mfa-section");
const uploadSection = document.getElementById("upload-section");

const usernameInput = document.getElementById("username");
const passwordInput = document.getElementById("password");
const loginButton   = document.getElementById("login-button");
const loginError    = document.getElementById("login-error");

const mfaCodeInput = document.getElementById("mfa-code");
const mfaButton    = document.getElementById("mfa-button");
const mfaError     = document.getElementById("mfa-error");

const dropzone   = document.getElementById("dropzone");
const fileInput  = document.getElementById("file-input");
const statusList = document.getElementById("status-list");

const fileListEl    = document.getElementById("file-list");
const refreshButton = document.getElementById("refresh-button");

// ----------------------------------------------------------------------------
// A tiny helper to call Cognito.
// ----------------------------------------------------------------------------
// Cognito's API works like this: you POST JSON to one URL, and you set a
// special header "X-Amz-Target" naming the action you want. This helper wraps
// that pattern so the two callers below stay short.
async function cognitoCall(action, payload) {
  // "await fetch(...)" sends the request and waits for the response.
  const response = await fetch(COGNITO_URL, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-amz-json-1.1", // the format Cognito expects
      "X-Amz-Target": `AWSCognitoIdentityProviderService.${action}`, // which action
    },
    body: JSON.stringify(payload), // turn our JS object into JSON text
  });

  // Read the JSON reply back into a JS object.
  const data = await response.json();

  // If Cognito returned an error status, throw so the caller's catch block
  // can show a message. data.message is Cognito's explanation.
  if (!response.ok) {
    throw new Error(data.message || "Login request failed");
  }
  return data;
}

// ----------------------------------------------------------------------------
// STEP 1: username + password
// ----------------------------------------------------------------------------
async function handleLogin() {
  loginError.textContent = ""; // clear any old error message
  pendingUsername = usernameInput.value.trim(); // .value is what was typed; trim removes stray spaces

  try {
    // Call InitiateAuth with the USER_PASSWORD_AUTH flow.
    const data = await cognitoCall("InitiateAuth", {
      AuthFlow: "USER_PASSWORD_AUTH",
      ClientId: CONFIG.clientId,
      AuthParameters: {
        USERNAME: pendingUsername,
        PASSWORD: passwordInput.value,
      },
    });

    // Because MFA is required, Cognito answers with a challenge rather than a
    // token. Check that it's the MFA challenge we expect.
    if (data.ChallengeName === "SOFTWARE_TOKEN_MFA") {
      mfaSession = data.Session; // save this; step 2 needs it
      // Hide the login boxes, show the MFA box.
      loginSection.classList.add("hidden");
      mfaSection.classList.remove("hidden");
      mfaCodeInput.focus(); // put the cursor in the code box for convenience
    } else if (data.AuthenticationResult) {
      // Unlikely (since MFA is on), but handle a straight success gracefully.
      finishLogin(data.AuthenticationResult.IdToken);
    } else {
      loginError.textContent = "Unexpected login response.";
    }
  } catch (err) {
    // Any thrown error lands here. .message is the human-readable text.
    loginError.textContent = err.message;
  }
}

// ----------------------------------------------------------------------------
// STEP 2: the MFA code
// ----------------------------------------------------------------------------
async function handleMfa() {
  mfaError.textContent = "";

  try {
    // Answer the challenge with the 6-digit code, including the saved Session.
    const data = await cognitoCall("RespondToAuthChallenge", {
      ChallengeName: "SOFTWARE_TOKEN_MFA",
      ClientId: CONFIG.clientId,
      Session: mfaSession,
      ChallengeResponses: {
        USERNAME: pendingUsername,
        SOFTWARE_TOKEN_MFA_CODE: mfaCodeInput.value.trim(),
      },
    });

    // On success Cognito returns the tokens. Grab the ID token and finish.
    finishLogin(data.AuthenticationResult.IdToken);
  } catch (err) {
    mfaError.textContent = err.message;
  }
}

// ----------------------------------------------------------------------------
// After a successful login: switch to the upload screen.
// ----------------------------------------------------------------------------
function finishLogin(token) {
  idToken = token; // remember the token for our upload requests
  mfaSection.classList.add("hidden");
  loginSection.classList.add("hidden");
  uploadSection.classList.remove("hidden");
  // As soon as we're logged in, load the list of existing files so the "Your
  // files" panel isn't empty. We don't "await" it -- let it load in the
  // background while the user can already start uploading.
  loadFileList();
}

// ----------------------------------------------------------------------------
// FILENAMES
// ----------------------------------------------------------------------------
// The API only accepts filenames matching [A-Za-z0-9._ -]. That is a
// deliberately tight allowlist on the server, and the server still enforces it
// -- but a browser is where people upload "résumé (final).pdf", and getting
// back a bare 400 with no explanation is a miserable experience. So we map a
// name into the accepted set here, up front, and tell the user we did it.
//
// This is a convenience, not a security control. Nothing here is trusted by
// the server; see lambda/handler.go for the check that actually matters.
function sanitizeFilename(name) {
  const clean = (s) =>
    s
      // "é" becomes "e" plus a combining accent, which the next step drops.
      // This turns accented letters into their plain ASCII base, not "-".
      .normalize("NFKD")
      .replace(/[\u0300-\u036f]/g, "")
      // Anything still outside the allowlist becomes a dash.
      .replace(/[^A-Za-z0-9._ -]/g, "-")
      // Collapse runs of dashes, and trim punctuation off the ends.
      .replace(/-{2,}/g, "-")
      .replace(/^[-. ]+/, "")
      .replace(/[-. ]+$/, "");

  // Clean the name and the extension separately, so a name written entirely in
  // characters outside the allowlist keeps its extension. Cleaning the whole
  // string at once turns "日本語.txt" into "txt": the name collapses to
  // nothing, the dot is trimmed as leading punctuation, and the extension is
  // left masquerading as the filename.
  //
  // dot > 0 rather than >= 0, because a leading dot marks a dotfile, not an
  // extension -- ".bashrc" should not become "upload.bashrc".
  const dot = name.lastIndexOf(".");

  let base = name;
  let ext = "";

  if (dot > 0) {
    base = name.slice(0, dot);
    ext = name.slice(dot + 1);
  }

  const cleanExt = clean(ext);
  const cleanBase = clean(base) || "upload";

  // The server also caps length at 255.
  const capped = (cleanExt ? `${cleanBase}.${cleanExt}` : cleanBase).slice(0, 255);

  // If a name was entirely outside the allowlist we would be left with nothing,
  // and an empty filename is rejected. Give it something rather than fail.
  return capped || "upload";
}

// ----------------------------------------------------------------------------
// UPLOADING
// ----------------------------------------------------------------------------
// Ask our own API for a one-time upload URL for a given file.
async function getUploadUrl(file, filename) {
  const response = await fetch(`${CONFIG.apiBaseUrl}/upload-url`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${idToken}`, // proves we're logged in
      "Content-Type": "application/json",
    },
    // file.type is the browser's guess at the file's type.
    body: JSON.stringify({ filename: filename, contentType: file.type }),
  });

  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || "Could not get an upload URL");
  }
  return data.uploadUrl;
}

// Upload a single file: get a URL, then PUT the file straight to S3.
async function uploadFile(file, statusEl) {
  // Work out the name the server will accept before we start, so we can both
  // ask for the right URL and tell the user if it changed.
  const filename = sanitizeFilename(file.name);

  try {
    statusEl.textContent = `Uploading ${file.name}...`;

    const uploadUrl = await getUploadUrl(file, filename);

    // PUT the raw file to the presigned URL. The browser sends the file
    // directly to S3 -- it does NOT pass through our Lambda.
    //
    // The content type has to match what the URL was minted for, or S3 rejects
    // the signature, so both sides use the same fallback.
    const putResponse = await fetch(uploadUrl, {
      method: "PUT",
      headers: { "Content-Type": file.type || "application/octet-stream" },
      body: file,
    });

    if (!putResponse.ok) {
      throw new Error(`S3 returned status ${putResponse.status}`);
    }

    statusEl.textContent = `Done: ${file.name}`;

    // If we had to change the name, say so -- otherwise the file appears in the
    // list below under a name the user never typed, which looks like a bug.
    if (filename !== file.name) {
      const note = document.createElement("span");
      note.className = "note";
      note.textContent = `  (saved as ${filename})`;
      statusEl.appendChild(note);
    }

    // Refresh the "Your files" panel so the file we just uploaded appears in
    // the list without the user having to click Refresh.
    loadFileList();
  } catch (err) {
    statusEl.textContent = `Failed: ${file.name} (${err.message})`;
    statusEl.classList.add("error");
  }
}

// Handle a batch of files (from either drag-drop or the file picker).
//
// The parameter is named "files", not "fileList": there is a fileListEl above
// pointing at the DOM node, and a parameter called fileList would shadow it
// inside this function.
function handleFiles(files) {
  // A FileList isn't a normal array, so we loop over it with a plain for loop.
  for (let i = 0; i < files.length; i++) {
    const file = files[i];
    // Create a status line for this file up front, so dropping 5 files shows
    // 5 rows immediately.
    const statusEl = document.createElement("div");
    statusEl.className = "status-item";
    statusEl.textContent = `Queued: ${file.name}`;
    statusList.appendChild(statusEl);

    // Start uploading. We don't "await" here on purpose -- letting them run
    // together means multiple files upload at the same time.
    uploadFile(file, statusEl);
  }
}

// ----------------------------------------------------------------------------
// WIRING: connect buttons and the drop zone to the functions above.
// ----------------------------------------------------------------------------
// This is the part that makes clicking/dragging actually DO something.

// Login button and pressing Enter in the password box both trigger login.
loginButton.addEventListener("click", handleLogin);
passwordInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") handleLogin();
});

// MFA button and Enter in the code box both trigger the MFA step.
mfaButton.addEventListener("click", handleMfa);
mfaCodeInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") handleMfa();
});

// Clicking the drop zone opens the (hidden) file picker.
dropzone.addEventListener("click", () => fileInput.click());

// When files are chosen via the picker, upload them.
fileInput.addEventListener("change", (e) => handleFiles(e.target.files));

// Drag-and-drop needs three event handlers:
// 1) dragover: we must "preventDefault" or the browser just opens the file.
dropzone.addEventListener("dragover", (e) => {
  e.preventDefault();
  dropzone.classList.add("dragging"); // highlight the zone
});
// 2) dragleave: remove the highlight when the file is dragged back out.
dropzone.addEventListener("dragleave", () => {
  dropzone.classList.remove("dragging");
});
// 3) drop: the moment of release -- grab the dropped files and upload them.
dropzone.addEventListener("drop", (e) => {
  e.preventDefault();
  dropzone.classList.remove("dragging");
  // e.dataTransfer.files is the list of files that were dropped.
  handleFiles(e.dataTransfer.files);
});

// ----------------------------------------------------------------------------
// BROWSING (list the files) and DOWNLOADING one
// ----------------------------------------------------------------------------

// Ask our API for the list of files in the bucket, then draw them on the page.
async function loadFileList() {
  // Show a placeholder while we fetch.
  fileListEl.textContent = "Loading...";

  try {
    // GET /files -- note there's no body for a GET; we just send our token.
    const response = await fetch(`${CONFIG.apiBaseUrl}/files`, {
      method: "GET",
      headers: { "Authorization": `Bearer ${idToken}` },
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Could not load file list");
    }

    // The API returns { "files": [ {key, size, lastModified}, ... ] }.
    // Clear the placeholder before drawing rows. Setting textContent to ""
    // empties the container.
    fileListEl.textContent = "";

    // If there are no files yet, say so and stop.
    if (data.files.length === 0) {
      fileListEl.textContent = "No files yet.";
      return;
    }

    // Draw one row per file. "for ... of" walks the list of file objects.
    for (const f of data.files) {
      // Make a container div for this row.
      const row = document.createElement("div");
      row.className = "status-item";

      // A text label showing the key and a human-friendly size.
      // Math.round(f.size / 1024) converts bytes to kilobytes, roughly.
      const label = document.createElement("span");
      label.textContent = `${f.key}  (${Math.round(f.size / 1024)} KB)  `;

      // A "Download" link. We don't know the download URL yet -- we fetch a
      // fresh presigned one only when the link is clicked (URLs expire, so
      // minting them lazily is best).
      const link = document.createElement("a");
      link.textContent = "Download";
      link.href = "#"; // a placeholder; the click handler does the real work
      // addEventListener("click", ...) runs our function when the link is
      // clicked. "async (e) =>" is an arrow function that can use await.
      link.addEventListener("click", async (e) => {
        e.preventDefault(); // stop the browser from following the "#" href
        await downloadFile(f.key);
      });

      // Put the label and link into the row, and the row into the list.
      row.appendChild(label);
      row.appendChild(link);
      fileListEl.appendChild(row);
    }
  } catch (err) {
    fileListEl.textContent = `Error: ${err.message}`;
  }
}

// Fetch a fresh presigned download URL for one key, then open it. Opening the
// URL causes the browser to download the file directly from S3.
async function downloadFile(key) {
  try {
    // POST /download-url with the key in the body.
    const response = await fetch(`${CONFIG.apiBaseUrl}/download-url`, {
      method: "POST",
      headers: {
        "Authorization": `Bearer ${idToken}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ key: key }),
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Could not get a download URL");
    }

    // window.open(url) opens the presigned URL. Because it's a direct S3 link,
    // the browser downloads (or displays) the file. "_blank" opens a new tab.
    //
    // This is a navigation rather than a fetch, which is why the uploads
    // bucket's CORS rules do not need to mention GET.
    window.open(data.downloadUrl, "_blank");
  } catch (err) {
    alert(`Download failed: ${err.message}`); // a simple popup for errors
  }
}

// Clicking "Refresh list" reloads the file list.
refreshButton.addEventListener("click", loadFileList);
