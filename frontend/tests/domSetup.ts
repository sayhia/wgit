import { GlobalWindow } from "happy-dom";

const dom = new GlobalWindow();
for (const key of Object.getOwnPropertyNames(dom)) {
  if (!(key in globalThis)) {
    try {
      (globalThis as any)[key] = (dom as any)[key];
    } catch {
      // ignore
    }
  }
}
(globalThis as any).window = dom;
(globalThis as any).document = dom.document;
(globalThis as any).navigator = dom.navigator;
(globalThis as any).localStorage = dom.localStorage;
(globalThis as any).getComputedStyle = dom.getComputedStyle.bind(dom);
(globalThis as any).customElements = dom.customElements;
(globalThis as any).Event = dom.Event;
(globalThis as any).CustomEvent = dom.CustomEvent;
(globalThis as any).EventTarget = dom.EventTarget;
(globalThis as any).UIEvent = dom.UIEvent;
(globalThis as any).MouseEvent = dom.MouseEvent;
(globalThis as any).KeyboardEvent = dom.KeyboardEvent;
