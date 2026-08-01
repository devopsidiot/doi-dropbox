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
// ============================================================================

// The web address of Cognito's API in your region, built from config.js.
const COGNITO_URL = `https://cognito-idp.${CONFIG.region}.amazonaws.com/`;

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
}

// ----------------------------------------------------------------------------
// UPLOADING
// ----------------------------------------------------------------------------
// Ask our own API for a one-time upload URL for a given file.
async function getUploadUrl(file) {
  const response = await fetch(`${CONFIG.apiBaseUrl}/upload-url`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${idToken}`, // proves we're logged in
      "Content-Type": "application/json",
    },
    // file.name is the filename; file.type is the browser's guess at its type.
    body: JSON.stringify({ filename: file.name, contentType: file.type }),
  });

  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || "Could not get an upload URL");
  }
  return data.uploadUrl;
}

// Upload a single file: get a URL, then PUT the file straight to S3.
async function uploadFile(file, statusEl) {
  try {
    statusEl.textContent = `Uploading ${file.name}...`;

    const uploadUrl = await getUploadUrl(file);

    // PUT the raw file to the presigned URL. The browser sends the file
    // directly to S3 -- it does NOT pass through our Lambda.
    const putResponse = await fetch(uploadUrl, {
      method: "PUT",
      headers: { "Content-Type": file.type || "application/octet-stream" },
      body: file,
    });

    if (!putResponse.ok) {
      throw new Error(`S3 returned status ${putResponse.status}`);
    }
    statusEl.textContent = `Done: ${file.name}`;
  } catch (err) {
    statusEl.textContent = `Failed: ${file.name} (${err.message})`;
    statusEl.classList.add("error");
  }
}

// Handle a batch of files (from either drag-drop or the file picker).
function handleFiles(fileList) {
  // A FileList isn't a normal array, so we loop over it with a plain for loop.
  for (let i = 0; i < fileList.length; i++) {
    const file = fileList[i];
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
