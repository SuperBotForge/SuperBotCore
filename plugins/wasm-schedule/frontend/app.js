const form = document.querySelector("#scheduleForm");
const coreUrlField = document.querySelector("#coreUrlField");
const coreUrlInput = document.querySelector("#coreUrl");
const roomInput = document.querySelector("#room");
const dateInput = document.querySelector("#date");
const localeInput = document.querySelector("#locale");
const statusBox = document.querySelector("#status");
const statusText = document.querySelector("#statusText");
const summary = document.querySelector("#summary");
const emptyState = document.querySelector("#emptyState");
const classes = document.querySelector("#classes");
const loginButton = document.querySelector("#loginButton");
const refreshButton = document.querySelector("#refreshButton");

const endpointPath = "/api/triggers/http/schedule/api/schedule";
const isBundled = window.location.pathname.startsWith("/plugins/");
const defaultCoreURL = isBundled ? window.location.origin : "http://127.0.0.1:4000";

dateInput.valueAsDate = new Date();
coreUrlInput.value = defaultCoreURL;
coreUrlField.hidden = isBundled;

form.addEventListener("submit", (event) => {
  event.preventDefault();
  loadSchedule();
});

refreshButton.addEventListener("click", () => loadSchedule());
loginButton.addEventListener("click", () => {
  const coreURL = resolveCoreURL();
  const loginURL = new URL("/api/auth/tsu/start", coreURL);
  loginURL.searchParams.set("return_to", loginReturnTo());
  window.location.href = loginURL.toString();
});

function selectedBuilding() {
  return new FormData(form).get("building")?.toString() || "1";
}

function normalizeCoreURL(value) {
  return value.trim().replace(/\/+$/, "") || defaultCoreURL;
}

function resolveCoreURL() {
  if (isBundled) {
    return window.location.origin;
  }
  return normalizeCoreURL(coreUrlInput.value);
}

function loginReturnTo() {
  if (!isBundled) {
    return window.location.href;
  }
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
}

function setStatus(kind, text) {
  statusBox.classList.remove("ok", "error", "loading");
  if (kind) statusBox.classList.add(kind);
  statusText.textContent = text;
}

function renderEmpty(text) {
  classes.innerHTML = "";
  emptyState.hidden = false;
  emptyState.textContent = text;
}

function renderSchedule(payload) {
  summary.textContent = `Building ${payload.building}, room ${payload.room}, ${payload.date}`;
  classes.innerHTML = "";

  const entries = Array.isArray(payload.classes) ? payload.classes : [];
  if (entries.length === 0) {
    renderEmpty("No classes found for this selection.");
    return;
  }

  emptyState.hidden = true;
  for (const item of entries) {
    const row = document.createElement("article");
    row.className = "class-row";
    row.innerHTML = `
      <div class="class-time"></div>
      <div>
        <div class="class-subject"></div>
        <div class="class-teacher"></div>
      </div>
    `;
    row.querySelector(".class-time").textContent = item.time || "";
    row.querySelector(".class-subject").textContent = item.subject || "";
    row.querySelector(".class-teacher").textContent = item.teacher || "";
    classes.append(row);
  }
}

async function loadSchedule() {
  const coreURL = resolveCoreURL();
  const url = new URL(endpointPath, coreURL);
  url.searchParams.set("building", selectedBuilding());
  url.searchParams.set("room", roomInput.value.trim() || "-");
  url.searchParams.set("date", dateInput.value || "today");
  url.searchParams.set("locale", localeInput.value);

  setStatus("loading", "Loading");
  try {
    const response = await fetch(url, {
      credentials: "include",
      headers: { Accept: "application/json" },
    });

    if (response.status === 401) {
      setStatus("error", "Login required");
      renderEmpty("The core did not receive a valid user_session cookie.");
      return;
    }
    if (response.status === 403) {
      setStatus("error", "Forbidden");
      renderEmpty("The current user is not allowed to call this trigger.");
      return;
    }
    if (!response.ok) {
      setStatus("error", `HTTP ${response.status}`);
      renderEmpty(await response.text());
      return;
    }

    const payload = await response.json();
    renderSchedule(payload);
    setStatus("ok", "Loaded");
  } catch (error) {
    setStatus("error", "Network/CORS error");
    renderEmpty(error instanceof Error ? error.message : "Request failed");
  }
}

loadSchedule();
