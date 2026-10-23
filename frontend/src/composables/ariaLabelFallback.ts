const SELECTOR = 'input:not([type="hidden"]), select, textarea';

function hasExplicitLabel(el: HTMLElement): boolean {
  if (el.hasAttribute("aria-labelledby")) return true;
  if (el.hasAttribute("aria-label") && el.dataset.autoAriaLabel !== "true") return true;
  const id = el.id;
  if (id && document.querySelector(`label[for="${CSS.escape(id)}"]`)) return true;
  return el.closest("label") !== null;
}

function labelControl(el: Element) {
  if (!(el instanceof HTMLElement) || !el.matches(SELECTOR) || hasExplicitLabel(el)) return;
  const placeholder = el.getAttribute("placeholder")?.replace(/\s+/g, " ").trim();
  if (!placeholder) return;
  el.setAttribute("aria-label", placeholder);
  el.dataset.autoAriaLabel = "true";
}

function labelTree(root: ParentNode) {
  if (root instanceof HTMLElement) labelControl(root);
  root.querySelectorAll?.(SELECTOR).forEach(labelControl);
}

// Progressive fallback for the long tail of placeholder-only form controls.
// Explicit labels/aria attributes always win and are never overwritten.
export function installAriaLabelFallback() {
  labelTree(document);
  new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      if (mutation.type === "attributes") {
        labelControl(mutation.target as Element);
      } else {
        mutation.addedNodes.forEach((node) => {
          if (node instanceof Element) labelTree(node);
        });
      }
    }
  }).observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["placeholder"],
  });
}
