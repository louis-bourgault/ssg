package dev

import (
	"strings"

	"golang.org/x/net/html"
)

const clientTag = `<script src="/__ssg/client.js" defer></script>`

const liveReloadClient = `(() => {
  "use strict";
  let socket = null;
  let reconnectTimer = null;
  let attempts = 0;
  let stopped = false;

  function showError(message) {
    let overlay = document.getElementById("__ssg-error");
    if (!overlay) {
      overlay = document.createElement("div");
      overlay.id = "__ssg-error";
      overlay.style.cssText = "position:fixed;z-index:2147483647;right:12px;bottom:12px;max-width:min(720px,calc(100vw - 24px));max-height:50vh;overflow:auto;background:#2b1114;color:#ffe8ea;border:1px solid #e35d6a;border-radius:6px;padding:12px 38px 12px 14px;box-shadow:0 4px 24px #0008;font:13px/1.45 ui-monospace,monospace;white-space:pre-wrap";
      const close = document.createElement("button");
      close.type = "button";
      close.textContent = "×";
      close.setAttribute("aria-label", "Dismiss build error");
      close.style.cssText = "position:absolute;top:4px;right:8px;border:0;background:transparent;color:inherit;font-size:22px;cursor:pointer";
      close.addEventListener("click", () => overlay.remove());
      overlay.appendChild(close);
      const text = document.createElement("div");
      text.className = "__ssg-error-text";
      overlay.appendChild(text);
      document.documentElement.appendChild(overlay);
    }
    overlay.querySelector(".__ssg-error-text").textContent = "Build failed\n\n" + message;
  }

  function clearError() {
    document.getElementById("__ssg-error")?.remove();
  }

  function refreshCSS(path) {
    const wanted = new URL(path, location.href).pathname;
    document.querySelectorAll('link[rel~="stylesheet"][href]').forEach((link) => {
      const current = new URL(link.href, location.href);
      if (current.pathname !== wanted) return;
      current.searchParams.set("__ssg", Date.now().toString());
      const replacement = link.cloneNode(true);
      replacement.href = current.href;
      replacement.addEventListener("load", () => link.remove(), { once: true });
      replacement.addEventListener("error", () => link.remove(), { once: true });
      link.after(replacement);
    });
  }

  function scheduleReconnect() {
    if (stopped || reconnectTimer !== null) return;
    const delay = Math.min(10000, 250 * (2 ** attempts));
    attempts = Math.min(attempts + 1, 6);
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null;
      connect();
    }, delay);
  }

  function connect() {
    if (stopped || socket !== null) return;
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    const url = protocol + "//" + location.host + "/__ssg/ws";
    const candidate = new WebSocket(url);
    socket = candidate;
    candidate.addEventListener("open", () => { attempts = 0; });
    candidate.addEventListener("message", (event) => {
      let message;
      try { message = JSON.parse(event.data); } catch (_) { return; }
      if (message.type === "reload") location.reload();
      else if (message.type === "css" && message.path) refreshCSS(message.path);
      else if (message.type === "error") showError(message.message || "Unknown build error");
      else if (message.type === "clear-error") clearError();
    });
    candidate.addEventListener("close", () => {
      if (socket === candidate) socket = null;
      scheduleReconnect();
    });
    candidate.addEventListener("error", () => candidate.close());
  }

  window.addEventListener("beforeunload", () => {
    stopped = true;
    if (reconnectTimer !== null) window.clearTimeout(reconnectTimer);
    reconnectTimer = null;
    socket?.close();
  });
  connect();
})();
`

func injectClientScript(document string) string {
	if hasHeadElement(document) {
		tokenizer := html.NewTokenizer(strings.NewReader(document))
		var output strings.Builder
		injected := false
		for {
			tokenType := tokenizer.Next()
			if tokenType == html.ErrorToken {
				return output.String()
			}
			output.Write(tokenizer.Raw())
			if !injected && tokenType == html.StartTagToken && tokenizer.Token().Data == "head" {
				output.WriteString(clientTag)
				injected = true
			}
		}
	}
	return clientTag + document
}

func hasHeadElement(document string) bool {
	tokenizer := html.NewTokenizer(strings.NewReader(document))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return false
		}
		if tokenType == html.StartTagToken && tokenizer.Token().Data == "head" {
			return true
		}
	}
}
