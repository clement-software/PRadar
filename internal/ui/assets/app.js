// Strict mode keeps Mermaid from executing embedded HTML or scripts.
if (window.mermaid) {
  const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  window.mermaid.initialize({ startOnLoad: true, securityLevel: "strict", theme: dark ? "dark" : "default" });
}
