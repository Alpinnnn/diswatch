const state = {
  config: null,
  statusTimer: null,
  toastTimeout: null,
};

const $ = (id) => document.getElementById(id);

// Toast notification system
function showToast(message, type = "success", duration = 4000) {
  const container = $("toast-container");
  const toast = document.createElement("div");
  toast.className = `toast ${type}`;
  toast.textContent = message;
  container.appendChild(toast);

  // Auto-remove after duration
  setTimeout(() => {
    toast.classList.add("fade-out");
    setTimeout(() => toast.remove(), 300);
  }, duration);
}

function clearToasts() {
  $("toast-container").innerHTML = "";
}

// API helper with better error handling
async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `Request failed: ${response.status}`);
  }
  return data;
}

function show(view) {
  $("auth-view").classList.toggle("hidden", view !== "auth");
  $("app-view").classList.toggle("hidden", view !== "app");
}

// Set button loading state
function setButtonLoading(buttonId, loading) {
  const btn = $(buttonId);
  if (!btn) return;
  btn.classList.toggle("loading", loading);
  btn.disabled = loading;
}

// Set all form buttons loading state
function setFormLoading(formId, loading) {
  const form = $(formId);
  if (!form) return;
  form.querySelectorAll("button[type='submit']").forEach(btn => {
    btn.classList.toggle("loading", loading);
    btn.disabled = loading;
  });
}

async function bootstrap() {
  try {
    const data = await api("/api/bootstrap");
    show("auth");
    $("setup-form").classList.toggle("hidden", data.initialized);
    $("login-form").classList.toggle("hidden", !data.initialized);

    // Display version in footer
    const versionDisplay = $("version-display");
    if (versionDisplay && data.version_full) {
      versionDisplay.textContent = data.version_full;
    } else if (versionDisplay && data.version) {
      versionDisplay.textContent = data.version;
    }
  } catch (error) {
    showToast("Failed to connect to server", "error");
  }
}

async function loadConfig() {
  try {
    state.config = await api("/api/config");
    fillForms(state.config);
    show("app");
    refreshStatus();
    if (!state.statusTimer) {
      state.statusTimer = setInterval(refreshStatus, 5000);
    }
    showToast("Connected", "success", 2000);
  } catch (error) {
    showToast("Session expired", "error");
    show("auth");
    bootstrap();
  }
}

function fillForms(cfg) {
  // Mode buttons - dual mode check
  const hasCustom = cfg.mode & 1; // ModeCustom bit
  const hasJellyfin = cfg.mode & 2; // ModeJellyfin bit

  // ModeCustom = 2 (0b10), ModeJellyfin = 1 (0b01)
  const customActive = (cfg.mode & 2) !== 0;
  const jellyfinActive = (cfg.mode & 1) !== 0;

  $("mode-custom").classList.toggle("active", customActive);
  $("mode-jellyfin").classList.toggle("active", jellyfinActive);

  // Show current mode
  if (customActive && jellyfinActive) {
    $("mode-state").textContent = "Dual";
  } else if (customActive) {
    $("mode-state").textContent = "Custom Only";
  } else if (jellyfinActive) {
    $("mode-state").textContent = "Jellyfin";
  } else {
    $("mode-state").textContent = "Unknown";
  }

  // Discord settings
  $("gateway-url").value = cfg.discord.gateway_url || "";
  $("application-id").value = cfg.discord.application_id || "";
  $("discord-status").value = cfg.discord.status || "online";

  if (cfg.discord.token_set) {
    $("discord-token-state").textContent = `Token saved (${cfg.discord.token_masked})`;
    $("discord-token-state").style.color = "var(--accent)";
  } else {
    $("discord-token-state").textContent = "No Discord token - RPC will not work";
    $("discord-token-state").style.color = "var(--danger)";
  }
  $("discord-token").value = "";
  $("clear-discord-token").checked = false;

  // Custom presence
  $("custom-name").value = cfg.custom.name || "";
  $("custom-type").value = String(cfg.custom.type ?? 0);
  $("custom-details").value = cfg.custom.details || "";
  $("custom-state").value = cfg.custom.state || "";
  $("large-image").value = cfg.custom.large_image || "";
  $("large-text").value = cfg.custom.large_text || "";
  $("small-image").value = cfg.custom.small_image || "";
  $("small-text").value = cfg.custom.small_text || "";
  $("details-url").value = cfg.custom.details_url || "";
  $("state-url").value = cfg.custom.state_url || "";
  $("use-elapsed").checked = Boolean(cfg.custom.use_elapsed);

  const buttons = cfg.custom.buttons || [];
  $("button1-label").value = buttons[0]?.label || "";
  $("button1-url").value = buttons[0]?.url || "";
  $("button2-label").value = buttons[1]?.label || "";
  $("button2-url").value = buttons[1]?.url || "";

  // Jellyfin settings
  $("jellyfin-url").value = cfg.jellyfin.url || "";
  $("jellyfin-api-key").value = "";

  if (cfg.jellyfin.api_key_set) {
    $("jellyfin-key-state").textContent = `API Key saved (${cfg.jellyfin.api_key_masked})`;
    $("jellyfin-key-state").style.color = "var(--accent)";
  } else {
    $("jellyfin-key-state").textContent = "No API key saved";
    $("jellyfin-key-state").style.color = "var(--muted)";
  }
  $("jellyfin-user-id").value = cfg.jellyfin.user_id || "";
  $("jellyfin-username").value = cfg.jellyfin.username || "";
  $("jellyfin-name").value = cfg.jellyfin.jellyfin_name || "";
  $("webhook-secret").value = cfg.jellyfin.webhook_secret || "";
  $("poll-interval").value = cfg.jellyfin.poll_interval_seconds || 30;
  $("clear-on-pause").checked = Boolean(cfg.jellyfin.clear_on_pause);
  $("clear-jellyfin-key").checked = false;
  $("webhook-url").textContent = `${location.origin}/api/jellyfin/webhook?secret=${encodeURIComponent(cfg.jellyfin.webhook_secret || "")}`;
}

async function refreshStatus() {
  try {
    const status = await api("/api/status");

    // Discord status with visual indicator
    const discordDot = $("discord-state").querySelector(".status-dot") || document.createElement("span");
    discordDot.className = "status-dot";

    if (status.discord.connected) {
      discordDot.classList.add("connected");
      $("discord-state").innerHTML = "";
      $("discord-state").appendChild(discordDot);
      $("discord-state").appendChild(document.createTextNode("Connected"));
    } else if (status.discord.configured) {
      discordDot.classList.add("disconnected");
      $("discord-state").innerHTML = "";
      $("discord-state").appendChild(discordDot);
      $("discord-state").appendChild(document.createTextNode("Disconnected"));
    } else {
      discordDot.classList.add("disconnected");
      $("discord-state").innerHTML = "";
      $("discord-state").appendChild(discordDot);
      $("discord-state").appendChild(document.createTextNode("Not configured"));
    }

    // Mode status
    const modeBits = status.mode;
    const hasCustom = (modeBits & 2) !== 0;
    const hasJellyfin = (modeBits & 1) !== 0;
    if (hasCustom && hasJellyfin) {
      $("mode-state").textContent = "Dual";
    } else if (hasCustom) {
      $("mode-state").textContent = "Custom Only";
    } else if (hasJellyfin) {
      $("mode-state").textContent = "Jellyfin";
    }

    // Jellyfin status with visual indicator
    const jfDot = $("jellyfin-state").querySelector(".status-dot") || document.createElement("span");
    jfDot.className = "status-dot";

    if (status.jellyfin.active) {
      jfDot.classList.add("active");
      $("jellyfin-state").innerHTML = "";
      $("jellyfin-state").appendChild(jfDot);
      const title = status.jellyfin.last_title || "media";
      $("jellyfin-state").appendChild(document.createTextNode(`Playing: ${title}`));
    } else if (status.jellyfin.last_error) {
      jfDot.classList.add("error");
      $("jellyfin-state").innerHTML = "";
      $("jellyfin-state").appendChild(jfDot);
      $("jellyfin-state").appendChild(document.createTextNode("Error"));
    } else {
      jfDot.classList.add("disconnected");
      $("jellyfin-state").innerHTML = "";
      $("jellyfin-state").appendChild(jfDot);
      $("jellyfin-state").appendChild(document.createTextNode("Idle"));
    }
  } catch {
    clearInterval(state.statusTimer);
    state.statusTimer = null;
    show("auth");
    bootstrap();
  }
}

function currentCustom() {
  const buttons = [];
  const button1 = { label: $("button1-label").value.trim(), url: $("button1-url").value.trim() };
  const button2 = { label: $("button2-label").value.trim(), url: $("button2-url").value.trim() };
  if (button1.label && button1.url) buttons.push(button1);
  if (button2.label && button2.url) buttons.push(button2);
  return {
    name: $("custom-name").value.trim(),
    type: Number($("custom-type").value),
    details: $("custom-details").value.trim(),
    state: $("custom-state").value.trim(),
    large_image: $("large-image").value.trim(),
    large_text: $("large-text").value.trim(),
    small_image: $("small-image").value.trim(),
    small_text: $("small-text").value.trim(),
    details_url: $("details-url").value.trim(),
    state_url: $("state-url").value.trim(),
    use_elapsed: $("use-elapsed").checked,
    buttons,
  };
}

async function saveConfig(payload, formId, successMsg) {
  setFormLoading(formId, true);
  try {
    const cfg = await api("/api/config", {
      method: "PUT",
      body: JSON.stringify(payload),
    });
    state.config = cfg;
    fillForms(cfg);
    showToast(successMsg, "success");
    refreshStatus();
  } catch (error) {
    showToast(error.message, "error");
  } finally {
    setFormLoading(formId, false);
  }
}

function wireEvents() {
  // Setup form
  $("setup-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const btn = $("setup-submit");
    setButtonLoading("setup-submit", true);
    try {
      await api("/api/setup", {
        method: "POST",
        body: JSON.stringify({ password: $("setup-password").value }),
      });
      await loadConfig();
      showToast("Dashboard created successfully", "success");
    } catch (error) {
      showToast(error.message, "error");
    } finally {
      setButtonLoading("setup-submit", false);
    }
  });

  // Login form
  $("login-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    setButtonLoading("login-submit", true);
    try {
      await api("/api/login", {
        method: "POST",
        body: JSON.stringify({ password: $("login-password").value }),
      });
      await loadConfig();
    } catch (error) {
      showToast(error.message, "error");
    } finally {
      setButtonLoading("login-submit", false);
    }
  });

  // Logout
  $("logout-button").addEventListener("click", async () => {
    clearToasts();
    clearInterval(state.statusTimer);
    state.statusTimer = null;
    await api("/api/logout", { method: "POST", body: "{}" });
    show("auth");
    bootstrap();
    showToast("Logged out", "info", 2000);
  });

  // Mode toggles
  $("mode-custom").addEventListener("click", () => switchMode("custom"));
  $("mode-jellyfin").addEventListener("click", () => switchMode("jellyfin"));

  // Discord form
  $("discord-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const discord = {
      gateway_url: $("gateway-url").value.trim(),
      status: $("discord-status").value,
      application_id: $("application-id").value.trim(),
      clear_token: $("clear-discord-token").checked,
    };
    const token = $("discord-token").value.trim();
    if (token) discord.token = token;
    saveConfig({ discord }, "discord-form", "Discord settings saved");
  });

  // Custom form
  $("custom-form").addEventListener("submit", (event) => {
    event.preventDefault();
    saveConfig({ custom: currentCustom() }, "custom-form", "Custom RPC saved");
  });

  // Jellyfin form
  $("jellyfin-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const jellyfin = {
      url: $("jellyfin-url").value.trim(),
      user_id: $("jellyfin-user-id").value.trim(),
      username: $("jellyfin-username").value.trim(),
      jellyfin_name: $("jellyfin-name").value.trim(),
      webhook_secret: $("webhook-secret").value.trim(),
      poll_interval_seconds: Number($("poll-interval").value),
      clear_on_pause: $("clear-on-pause").checked,
      clear_api_key: $("clear-jellyfin-key").checked,
    };
    const apiKey = $("jellyfin-api-key").value.trim();
    if (apiKey) jellyfin.api_key = apiKey;
    saveConfig({ jellyfin }, "jellyfin-form", "Jellyfin settings saved");
  });

  // Reconnect button
  $("reconnect-button").addEventListener("click", async () => {
    setButtonLoading("reconnect-button", true);
    try {
      await api("/api/discord/reconnect", { method: "POST", body: "{}" });
      showToast("Reconnecting to Discord...", "info");
      // Wait a moment then refresh status
      setTimeout(refreshStatus, 2000);
    } catch (error) {
      showToast(error.message, "error");
    } finally {
      setTimeout(() => setButtonLoading("reconnect-button", false), 2000);
    }
  });

  // Test Jellyfin button
  $("test-jellyfin-button").addEventListener("click", async () => {
    setButtonLoading("test-jellyfin-button", true);
    try {
      await api("/api/jellyfin/test", { method: "POST", body: "{}" });
      showToast("Jellyfin connection OK", "success");
    } catch (error) {
      showToast(`Jellyfin test failed: ${error.message}`, "error");
    } finally {
      setButtonLoading("test-jellyfin-button", false);
    }
  });
}

async function switchMode(mode) {
  try {
    const cfg = await api("/api/mode", {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
    state.config = cfg;
    fillForms(cfg);

    // Determine mode name for toast
    const customActive = (cfg.mode & 2) !== 0;
    const jellyfinActive = (cfg.mode & 1) !== 0;
    let modeName = "Dual";
    if (customActive && !jellyfinActive) modeName = "Custom Only";
    else if (jellyfinActive && !customActive) modeName = "Jellyfin";

    showToast(`Mode: ${modeName}`, "info");
    refreshStatus();
  } catch (error) {
    showToast(error.message, "error");
  }
}

// Initialize
wireEvents();
bootstrap().catch((error) => showToast(error.message, "error"));
