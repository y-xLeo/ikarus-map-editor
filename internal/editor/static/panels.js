/* ─────────────────────────────────────────────────────────────
   panels.js — dock chrome controller
   - rail-button → toggle floating panel
   - drag panels from their header
   - persist panel position in localStorage
   - paint range-slider gradient fill (via --val custom property)
   - keep rail-button active state in sync with panel visibility
   ───────────────────────────────────────────────────────────── */

(function () {
  "use strict";

  const STORAGE_PREFIX = "sromap.dock.";
  const RAIL_BUTTONS = Array.from(document.querySelectorAll(".rail-btn[data-panel]"));
  const DOCK_PANELS = Array.from(document.querySelectorAll(".dock-panel"));

  // ── Utilities ──────────────────────────────────────────────
  const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

  function savedKeyFor(panel) { return STORAGE_PREFIX + (panel.id || ""); }

  function savePosition(panel) {
    try {
      const s = panel.style;
      if (!s.left || !s.top) return;
      localStorage.setItem(savedKeyFor(panel), JSON.stringify({ left: s.left, top: s.top }));
    } catch (_) { /* ignore */ }
  }

  function loadPosition(panel) {
    try {
      const raw = localStorage.getItem(savedKeyFor(panel));
      if (!raw) return false;
      const v = JSON.parse(raw);
      if (!v || typeof v.left !== "string" || typeof v.top !== "string") return false;
      panel.style.left = v.left;
      panel.style.top = v.top;
      panel.style.right = "auto";
      panel.dataset.positioned = "1";
      return true;
    } catch (_) { return false; }
  }

  // ── Drag handling for any dock-panel with [data-drag-handle] header ──
  function makeDraggable(panel) {
    const header = panel.querySelector(".dock-header[data-drag-handle]");
    if (!header) return;

    let drag = null;

    header.addEventListener("mousedown", (e) => {
      if (e.button !== 0) return;
      // Don't initiate drag when clicking interactive elements inside the header
      if (e.target.closest("button, input, select, [data-close-panel]")) return;
      const rect = panel.getBoundingClientRect();
      drag = { offX: e.clientX - rect.left, offY: e.clientY - rect.top };
      panel.classList.add("dragging");
      bringToFront(panel);
      e.preventDefault();
    });

    window.addEventListener("mousemove", (e) => {
      if (!drag) return;
      const rect = panel.getBoundingClientRect();
      const maxX = window.innerWidth - rect.width;
      const maxY = window.innerHeight - rect.height;
      const x = clamp(e.clientX - drag.offX, 0, Math.max(0, maxX));
      const y = clamp(e.clientY - drag.offY, 0, Math.max(0, maxY));
      panel.style.left = `${x}px`;
      panel.style.top = `${y}px`;
      panel.style.right = "auto";
      panel.dataset.positioned = "1";
    });

    window.addEventListener("mouseup", () => {
      if (!drag) return;
      drag = null;
      panel.classList.remove("dragging");
      savePosition(panel);
    });
  }

  // ── Z-order: when a panel is opened or grabbed, raise it above siblings ──
  let zCounter = 30;
  function bringToFront(panel) {
    zCounter += 1;
    panel.style.zIndex = String(zCounter);
  }

  // ── Default position: cascade newly-opened panels so they don't overlap ──
  const railWidthVar = getComputedStyle(document.documentElement).getPropertyValue("--rail-w").trim() || "60px";
  const topbarVar = getComputedStyle(document.documentElement).getPropertyValue("--topbar-h").trim() || "56px";

  function placeDefault(panel) {
    if (panel.dataset.positioned) return;
    // Count panels already visible (excluding this one). Use that to stagger.
    const visibleCount = DOCK_PANELS.filter(p => p !== panel && !p.hidden).length;
    const offset = visibleCount * 22;
    panel.style.right = `calc(${railWidthVar} + 28px)`;
    panel.style.top = `calc(${topbarVar} + ${14 + offset}px)`;
    panel.style.left = "";
  }

  // ── Show / hide ──
  // Panels can opt in to a "first-shown" hook by id — used today to lazy-load
  // the tile catalog the first time the Tiles dock opens.
  const ON_FIRST_SHOW = {
    tilesPanel: () => window.ensureTileCatalog && window.ensureTileCatalog(),
  };

  function showPanel(panel) {
    if (!loadPosition(panel)) placeDefault(panel);
    panel.hidden = false;
    bringToFront(panel);
    const hook = ON_FIRST_SHOW[panel.id];
    if (hook) hook();
  }
  function hidePanel(panel) {
    panel.hidden = true;
  }
  function togglePanel(panel) {
    if (panel.hidden) showPanel(panel);
    else hidePanel(panel);
  }

  // ── Rail wiring ──
  function openWorldMap() {
    if (typeof window.openWorldMap === "function") {
      window.openWorldMap();
    } else {
      // app.js listens for raw key events on window; dispatch a synthetic M keydown
      window.dispatchEvent(new KeyboardEvent("keydown", {
        key: "m", code: "KeyM", bubbles: true
      }));
    }
  }

  function handleRailClick(btn) {
    const target = btn.dataset.panel;
    if (target === "worldMapTrigger") {
      openWorldMap();
      return;
    }
    const panel = document.getElementById(target);
    if (!panel) return;
    togglePanel(panel);
  }

  RAIL_BUTTONS.forEach(btn => {
    btn.addEventListener("click", () => handleRailClick(btn));
  });

  // close buttons inside each dock-panel
  document.querySelectorAll(".dock-close[data-close-panel]").forEach(btn => {
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      const panel = btn.closest(".dock-panel");
      if (panel) hidePanel(panel);
    });
  });

  // ── Active-state sync ──
  function syncRailActive() {
    const wm = document.getElementById("worldMap");
    for (const btn of RAIL_BUTTONS) {
      const id = btn.dataset.panel;
      if (id === "worldMapTrigger") {
        btn.classList.toggle("active", !!(wm && !wm.hidden));
      } else {
        const p = document.getElementById(id);
        btn.classList.toggle("active", !!(p && !p.hidden));
      }
    }
  }

  // observe every dock-panel's `hidden` attribute (also covers app.js toggling)
  const obs = new MutationObserver(syncRailActive);
  DOCK_PANELS.forEach(p => obs.observe(p, { attributes: true, attributeFilter: ["hidden"] }));
  const wm = document.getElementById("worldMap");
  if (wm) obs.observe(wm, { attributes: true, attributeFilter: ["hidden"] });

  // ── Slider gradient fill ──
  function updateSliderVar(input) {
    const min = parseFloat(input.min || "0");
    const max = parseFloat(input.max || "100");
    const v = parseFloat(input.value || "0");
    if (!isFinite(min) || !isFinite(max) || max === min) {
      input.style.setProperty("--val", "50%");
      return;
    }
    const pct = clamp(((v - min) / (max - min)) * 100, 0, 100);
    input.style.setProperty("--val", pct + "%");
  }
  function watchSlider(input) {
    updateSliderVar(input);
    input.addEventListener("input", () => updateSliderVar(input));
    input.addEventListener("change", () => updateSliderVar(input));
  }
  document.querySelectorAll("input[type=range]").forEach(watchSlider);

  // Slider values may also be programmatically changed by app.js (e.g. when
  // setting state from a load). Re-poll once after a short delay and once
  // when the window first reports idle so the gradient catches initial values.
  function refreshAllSliders() {
    document.querySelectorAll("input[type=range]").forEach(updateSliderVar);
  }
  if (document.readyState === "complete") {
    setTimeout(refreshAllSliders, 50);
  } else {
    window.addEventListener("load", () => setTimeout(refreshAllSliders, 50));
  }

  // ── Initialise: enable drag on every dock-panel (incl. objectPanel) ──
  // app.js already binds its own drag handler to #objectPanelHeader. To avoid
  // double-handling, skip #objectPanel here.
  DOCK_PANELS.forEach(panel => {
    if (panel.id !== "objectPanel") {
      makeDraggable(panel);
      loadPosition(panel);
    }
  });

  syncRailActive();

  // ── Focus the loaded panel when its rail-button is clicked again ──
  // (Convenience: bringing a hidden-then-shown panel forward.)
  RAIL_BUTTONS.forEach(btn => {
    btn.addEventListener("mousedown", () => {
      const id = btn.dataset.panel;
      if (id === "worldMapTrigger") return;
      const p = document.getElementById(id);
      if (p && !p.hidden) bringToFront(p);
    });
  });

  // Expose a tiny API for app.js to nudge things later if needed
  window.SromapPanels = {
    showPanel: (id) => { const p = document.getElementById(id); if (p) showPanel(p); },
    hidePanel: (id) => { const p = document.getElementById(id); if (p) hidePanel(p); },
    togglePanel: (id) => { const p = document.getElementById(id); if (p) togglePanel(p); },
    refreshSliders: refreshAllSliders,
  };
})();
