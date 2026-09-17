// Strict mode keeps Mermaid from executing embedded HTML or scripts.
if (window.mermaid) {
  const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  window.mermaid.initialize({ startOnLoad: true, securityLevel: "strict", theme: dark ? "dark" : "default" });
}

// One-minute comprehension timer: the elapsed time is measured client side and
// submitted with the scorecard; a manual seconds field remains as fallback.
for (const form of document.querySelectorAll("form[data-timer]")) {
  const start = form.querySelector("[data-start]");
  const output = form.querySelector("[data-elapsed]");
  const hidden = form.querySelector("[data-elapsed-ms]");
  const manual = form.querySelector("[data-elapsed-seconds]");
  let startedAt = 0;
  let ticker = 0;
  start.addEventListener("click", () => {
    startedAt = Date.now();
    clearInterval(ticker);
    ticker = setInterval(() => { output.textContent = Math.round((Date.now() - startedAt) / 1000) + " s"; }, 500);
  });
  form.addEventListener("submit", () => {
    clearInterval(ticker);
    if (startedAt) {
      hidden.value = String(Date.now() - startedAt);
    } else if (manual.value) {
      hidden.value = String(Number(manual.value) * 1000);
    }
  });
}
