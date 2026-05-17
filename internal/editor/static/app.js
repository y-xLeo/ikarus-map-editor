const REGION_SIZE = 1920;
const CELL_SIZE = 20;
const GRID_SIZE = 97;

const canvas = document.getElementById("view");
const gl = canvas.getContext("webgl", { antialias: true });
if (!gl) {
  document.getElementById("status").textContent = "WebGL is not available";
  throw new Error("WebGL is not available");
}

const ui = {
  status: document.getElementById("status"),
  regionX: document.getElementById("regionX"),
  regionY: document.getElementById("regionY"),
  loadRadius: document.getElementById("loadRadius"),
  loadBtn: document.getElementById("loadBtn"),
  streamingToggle: document.getElementById("streamingToggle"),
  worldMap: document.getElementById("worldMap"),
  worldMapCanvas: document.getElementById("worldMapCanvas"),
  worldMapCoords: document.getElementById("worldMapCoords"),
  worldMapClose: document.getElementById("worldMapClose"),
  worldMapMode: document.getElementById("worldMapMode"),
  worldMapActions: document.getElementById("worldMapActions"),
  worldMapSelInfo: document.getElementById("worldMapSelInfo"),
  worldMapCopyBtn: document.getElementById("worldMapCopyBtn"),
  worldMapCreateBtn: document.getElementById("worldMapCreateBtn"),
  worldMapDeleteBtn: document.getElementById("worldMapDeleteBtn"),
  worldMapClearSelBtn: document.getElementById("worldMapClearSelBtn"),
  worldMapPasteBar: document.getElementById("worldMapPasteBar"),
  worldMapPasteInfo: document.getElementById("worldMapPasteInfo"),
  worldMapCancelPasteBtn: document.getElementById("worldMapCancelPasteBtn"),
  worldMapHint: document.getElementById("worldMapHint"),
  createRegionDialog: document.getElementById("createRegionDialog"),
  createRegionHeight: document.getElementById("createRegionHeight"),
  createRegionSummary: document.getElementById("createRegionSummary"),
  deleteRegionDialog: document.getElementById("deleteRegionDialog"),
  deleteRegionSummary: document.getElementById("deleteRegionSummary"),
  pasteRegionDialog: document.getElementById("pasteRegionDialog"),
  pasteRegionSummary: document.getElementById("pasteRegionSummary"),
  pasteRegionFloor: document.getElementById("pasteRegionFloor"),
  pasteRegionRotation: document.getElementById("pasteRegionRotation"),
  pasteRegionFlipX: document.getElementById("pasteRegionFlipX"),
  pasteRegionFlipZ: document.getElementById("pasteRegionFlipZ"),
  raiseMode: document.getElementById("raiseMode"),
  lowerMode: document.getElementById("lowerMode"),
  equalizeMode: document.getElementById("equalizeMode"),
  equalizeStatus: document.getElementById("equalizeStatus"),
  brushRadius: document.getElementById("brushRadius"),
  brushStrength: document.getElementById("brushStrength"),
  radiusOut: document.getElementById("radiusOut"),
  strengthOut: document.getElementById("strengthOut"),
  saveAllBtn: document.getElementById("saveAllBtn"),
  loadedCount: document.getElementById("loadedCount"),
  dirtyCount: document.getElementById("dirtyCount"),
  objectCount: document.getElementById("objectCount"),
  activeCount: document.getElementById("activeCount"),
  refText: document.getElementById("refText"),
  exportDir: document.getElementById("exportDir"),
  lightmapStrength: document.getElementById("lightmapStrength"),
  lightmapStrengthOut: document.getElementById("lightmapStrengthOut"),
  bakeShadowsBtn: document.getElementById("bakeShadowsBtn"),
  restoreLightmapBtn: document.getElementById("restoreLightmapBtn"),
  rebuildNVMBtn: document.getElementById("rebuildNVMBtn"),
  restoreNVMBtn: document.getElementById("restoreNVMBtn"),
  showNVMOverlay: document.getElementById("showNVMOverlay"),
  nvmSlope: document.getElementById("nvmSlope"),
  nvmSlopeOut: document.getElementById("nvmSlopeOut"),
  nvmCreateMissing: document.getElementById("nvmCreateMissing"),
  bakeStatus: document.getElementById("bakeStatus"),
  cursorCoords: document.getElementById("cursorCoords"),
  bakeAzimuth: document.getElementById("bakeAzimuth"),
  bakeAzimuthOut: document.getElementById("bakeAzimuthOut"),
  bakeElevation: document.getElementById("bakeElevation"),
  bakeElevationOut: document.getElementById("bakeElevationOut"),
  bakeSamples: document.getElementById("bakeSamples"),
  bakeSamplesOut: document.getElementById("bakeSamplesOut"),
  bakeSoftness: document.getElementById("bakeSoftness"),
  bakeSoftnessOut: document.getElementById("bakeSoftnessOut"),
  brushToolMode: document.getElementById("brushToolMode"),
  selectToolMode: document.getElementById("selectToolMode"),
  tileMode: document.getElementById("tileMode"),
  tilePickerSection: document.getElementById("tilePickerSection"),
  tileFilter: document.getElementById("tileFilter"),
  tileGrid: document.getElementById("tileGrid"),
  tileStatus: document.getElementById("tileStatus"),
  brushControls: document.getElementById("brushControls"),
  selectControls: document.getElementById("selectControls"),
  objectPanel: document.getElementById("objectPanel"),
  objectPanelHeader: document.getElementById("objectPanelHeader"),
  selObjName: document.getElementById("selObjName"),
  selObjMeta: document.getElementById("selObjMeta"),
  objectEdit: document.getElementById("objectEdit"),
  objXSlider: document.getElementById("objXSlider"),
  objXInput: document.getElementById("objXInput"),
  objYSlider: document.getElementById("objYSlider"),
  objYInput: document.getElementById("objYInput"),
  objZSlider: document.getElementById("objZSlider"),
  objZInput: document.getElementById("objZInput"),
  objYawSlider: document.getElementById("objYawSlider"),
  objYawInput: document.getElementById("objYawInput"),
  snapToTerrainBtn: document.getElementById("snapToTerrainBtn"),
  resetObjectBtn: document.getElementById("resetObjectBtn"),
  deleteObjectBtn: document.getElementById("deleteObjectBtn"),
  showObjectCollision: document.getElementById("showObjectCollision"),
  showHandoffOverlay: document.getElementById("showHandoffOverlay"),
  objectSaveStatus: document.getElementById("objectSaveStatus"),
  spawnSection: document.getElementById("spawnSection"),
  spawnFilter: document.getElementById("spawnFilter"),
  spawnList: document.getElementById("spawnList"),
  importCustomObjectBtn: document.getElementById("importCustomObjectBtn"),
  importCustomObjectDialog: document.getElementById("importCustomObjectDialog"),
  importCustomFolder: document.getElementById("importCustomFolder"),
  importCustomName: document.getElementById("importCustomName"),
  importCustomSize: document.getElementById("importCustomSize"),
  spawnStatus: document.getElementById("spawnStatus")
};

const state = {
  info: null,
  baseX: 0,
  baseY: 0,
  regions: new Map(),
  dirty: new Set(),
  objects: { count: 0, position: null, color: null },
  objectAssets: new Map(),     // objID -> { meshes:[{vbuf,ibuf,texture,count}], state, bbox }
  objectPlacements: [],        // { objID, uid, regionID, regionX, regionY, originalX/Y/Z, wx, wy, wz, yaw, objectPath, isNew? }
  dirtyPlacements: new Set(),  // indices in objectPlacements that have been edited
  pendingDeletes: [],          // [{objID, uid, regionID}] of placements removed from array
  objectCatalog: null,         // fetched object.ifo list
  customCatalog: null,         // imported OBJ list (high-ID range)
  spawnFilterText: "",
  tileCatalog: null,           // fetched tile2d.ifo list
  tileFilterText: "",
  activeTileID: null,          // selected tile ID for the Tile brush
  tileDirty: new Set(),        // region keys that had tile changes
  selection: null,             // { index: number } pointing into objectPlacements
  dragging: null,              // { planeY, offsetX, offsetZ, index } while dragging a selection
  toolMode: "brush",           // "brush" | "select"
  lightmapStrength: 0.6,       // 0 = no lightmap, 1 = full bake
  showNVMOverlay: false,
  showObjectCollision: false,
  showHandoffOverlay: false,
  undoStack: [],
  redoStack: [],
  undoLimit: 100,
  nextClientID: 1,
  editSession: null,           // { cid, label, before } in-progress placement edit
  editIdleTimer: 0,
  brushStroke: null,           // { mode, regionDeltas: Map<key, Map<idx, oldValue>> }
  hover: null,
  brushMode: "raise",
  equalizeTarget: null,            // sampled terrain height for equalize brush
  lastBrushAt: 0,
  leftDown: false,
  rightDown: false,
  keys: new Set(),
  mouse: { x: 0, y: 0 },
  camera: { x: 0, y: 700, z: 2200, yaw: Math.PI, pitch: -0.35 },
  lastFrame: performance.now(),
  // Dynamic region streaming
  streaming: true,
  streamingInFlight: new Set(),   // "x,y" of regions currently being loaded
  lastCamRegion: null,            // {x,y} of camera's containing region last check
  // World-map overlay
  worldMapOpen: false,
  worldMapHover: null,            // {x, y} of region hovered in minimap
  worldMapMode: "navigate",       // "navigate" | "select"
  worldMapDrag: null,             // { ax, ay, bx, by } in region coords during drag
  worldMapSelection: null,        // committed {minX, maxX, minY, maxY}
  worldMapZoom: 1,                // multiplier on WORLD_MAP_PX
  worldMapPanX: 0,                // canvas-pixel offset
  worldMapPanY: 0,
  worldMapPanDrag: null,          // { x, y, panX, panY } while right-dragging
  // Clipboard: source rect of active regions captured for paste
  worldMapClipboard: null         // { tiles: [{x,y}], minX, minY, w, h }
};

const terrainProgram = createProgram(`
attribute vec3 aPosition;
attribute vec3 aColor;
attribute vec3 aNormal;
attribute vec2 aTexCoord;
uniform mat4 uViewProj;
varying vec3 vColor;
varying vec2 vUV;
varying float vLight;
void main() {
  vColor = aColor;
  vUV = aTexCoord;
  vec3 lightDir = normalize(vec3(0.4, 0.85, 0.3));
  vLight = 0.78 + 0.22 * max(dot(normalize(aNormal), lightDir), 0.0);
  gl_Position = uViewProj * vec4(aPosition, 1.0);
}
`, `
precision mediump float;
varying vec3 vColor;
varying vec2 vUV;
varying float vLight;
uniform sampler2D uTexture;
uniform sampler2D uLightmap;
uniform float uHasTexture;
uniform float uLightmapStrength;
void main() {
  vec3 textured = texture2D(uTexture, vUV).rgb;
  vec3 base = mix(vColor, textured, uHasTexture);
  vec3 lightmap = texture2D(uLightmap, vUV).rgb;
  vec3 shadow = mix(vec3(1.0), lightmap, uLightmapStrength);
  gl_FragColor = vec4(base * vLight * shadow, 1.0);
}
`);
const whitePixelTexture = createWhitePixelTexture();

const pointProgram = createProgram(`
attribute vec3 aPosition;
attribute vec3 aColor;
uniform mat4 uViewProj;
varying vec3 vColor;
void main() {
  vColor = aColor;
  gl_Position = uViewProj * vec4(aPosition, 1.0);
  gl_PointSize = 8.0;
}
`, `
precision mediump float;
varying vec3 vColor;
void main() {
  vec2 p = gl_PointCoord - vec2(0.5);
  if (dot(p, p) > 0.25) discard;
  gl_FragColor = vec4(vColor, 1.0);
}
`);

const lineProgram = createProgram(`
attribute vec3 aPosition;
uniform mat4 uViewProj;
uniform vec3 uColor;
varying vec3 vColor;
void main() {
  vColor = uColor;
  gl_Position = uViewProj * vec4(aPosition, 1.0);
}
`, `
precision mediump float;
varying vec3 vColor;
void main() {
  gl_FragColor = vec4(vColor, 1.0);
}
`);

const flatProgram = createProgram(`
attribute vec3 aPosition;
uniform mat4 uViewProj;
void main() {
  gl_Position = uViewProj * vec4(aPosition, 1.0);
}
`, `
precision mediump float;
uniform vec4 uColor;
void main() {
  gl_FragColor = uColor;
}
`);

const objectProgram = createProgram(`
attribute vec3 aPosition;
attribute vec2 aTexCoord;
uniform mat4 uMVP;
varying vec2 vUV;
void main() {
  vUV = aTexCoord;
  gl_Position = uMVP * vec4(aPosition, 1.0);
}
`, `
precision mediump float;
varying vec2 vUV;
uniform sampler2D uTexture;
uniform float uHasTexture;
uniform vec3 uTint;
void main() {
  vec4 sampled = texture2D(uTexture, vUV);
  if (uHasTexture > 0.5 && sampled.a < 0.35) discard;
  vec3 base = mix(uTint, sampled.rgb, uHasTexture);
  gl_FragColor = vec4(base, 1.0);
}
`);

const terrainIndexBuffer = gl.createBuffer();
const terrainIndexCount = buildTerrainIndices();
const brushBuffer = gl.createBuffer();
const objectPositionBuffer = gl.createBuffer();
const objectColorBuffer = gl.createBuffer();
const selectionBuffer = gl.createBuffer();
const collisionFillBuffer = gl.createBuffer();
const collisionLineBuffer = gl.createBuffer();
const handoffOpenFillBuffer = gl.createBuffer();
const handoffClosedFillBuffer = gl.createBuffer();
const handoffOpenLineBuffer = gl.createBuffer();
const handoffClosedLineBuffer = gl.createBuffer();

gl.enable(gl.DEPTH_TEST);
gl.disable(gl.CULL_FACE);
gl.clearColor(0.42, 0.58, 0.72, 1);

ui.loadBtn.addEventListener("click", () => loadSelectedRegion());
ui.saveAllBtn.addEventListener("click", () => saveAll());
ui.streamingToggle.addEventListener("change", () => {
  state.streaming = ui.streamingToggle.checked;
  if (state.streaming) state.lastCamRegion = null; // force a tick
});
ui.worldMapClose.addEventListener("click", () => toggleWorldMap(false));
ui.worldMapMode.addEventListener("click", () => {
  setWorldMapMode(state.worldMapMode === "navigate" ? "select" : "navigate");
});
ui.worldMapClearSelBtn.addEventListener("click", () => {
  state.worldMapSelection = null;
  refreshWorldMapActions();
});
ui.worldMapCreateBtn.addEventListener("click", openCreateRegionDialog);
ui.worldMapDeleteBtn.addEventListener("click", openDeleteRegionDialog);
ui.worldMapCopyBtn.addEventListener("click", copyWorldMapSelection);
ui.worldMapCancelPasteBtn.addEventListener("click", clearWorldMapClipboard);
ui.deleteRegionDialog.addEventListener("close", onDeleteRegionDialogClose);
ui.pasteRegionDialog.addEventListener("close", onPasteRegionDialogClose);
ui.worldMapCanvas.addEventListener("mousedown", onWorldMapMouseDown);
ui.worldMapCanvas.addEventListener("mousemove", onWorldMapMouseMove);
ui.worldMapCanvas.addEventListener("mouseleave", () => {
  state.worldMapHover = null;
  ui.worldMapCoords.textContent = "-";
});
ui.worldMapCanvas.addEventListener("wheel", onWorldMapWheel, { passive: false });
ui.worldMapCanvas.addEventListener("contextmenu", e => e.preventDefault());
window.addEventListener("mouseup", onWorldMapMouseUp);
ui.createRegionDialog.addEventListener("close", onCreateRegionDialogClose);
ui.raiseMode.addEventListener("click", () => setBrushMode("raise"));
ui.lowerMode.addEventListener("click", () => setBrushMode("lower"));
ui.equalizeMode.addEventListener("click", () => setBrushMode("equalize"));
ui.tileMode.addEventListener("click", () => setBrushMode("tile"));
ui.tileFilter.addEventListener("input", () => {
  state.tileFilterText = ui.tileFilter.value;
  renderTileGrid();
});
ui.brushRadius.addEventListener("input", updateBrushLabels);
ui.brushStrength.addEventListener("input", updateBrushLabels);
ui.lightmapStrength.addEventListener("input", () => {
  state.lightmapStrength = parseFloat(ui.lightmapStrength.value);
  ui.lightmapStrengthOut.textContent = state.lightmapStrength.toFixed(2);
});
ui.lightmapStrengthOut.textContent = ui.lightmapStrength.value;
state.lightmapStrength = parseFloat(ui.lightmapStrength.value);

ui.bakeShadowsBtn.addEventListener("click", bakeLoadedRegionShadows);
ui.restoreLightmapBtn.addEventListener("click", restoreLoadedRegionLightmaps);
ui.rebuildNVMBtn.addEventListener("click", rebuildLoadedRegionNVMs);
ui.restoreNVMBtn.addEventListener("click", restoreLoadedRegionNVMs);
ui.nvmSlope.addEventListener("input", () => { ui.nvmSlopeOut.textContent = ui.nvmSlope.value; });
ui.nvmSlopeOut.textContent = ui.nvmSlope.value;
ui.showNVMOverlay.addEventListener("change", () => {
  state.showNVMOverlay = ui.showNVMOverlay.checked;
});
ui.showObjectCollision.addEventListener("change", () => {
  state.showObjectCollision = ui.showObjectCollision.checked;
});
ui.showHandoffOverlay.addEventListener("change", () => {
  state.showHandoffOverlay = ui.showHandoffOverlay.checked;
  if (state.showHandoffOverlay) state.showNVMOverlay = true;
  ui.showNVMOverlay.checked = state.showNVMOverlay;
});

// Persist tuning sliders across reloads
for (const [el, key] of [
  [ui.bakeAzimuth, "bakeAzimuth"],
  [ui.bakeElevation, "bakeElevation"],
  [ui.bakeSamples, "bakeSamples"],
  [ui.bakeSoftness, "bakeSoftness"],
  [ui.lightmapStrength, "lightmapStrength"],
  [ui.nvmSlope, "nvmSlope"]
]) {
  try {
    const saved = localStorage.getItem(`sromap.${key}`);
    if (saved !== null) {
      el.value = saved;
      el.dispatchEvent(new Event("input"));
    }
  } catch (_) { /* ignore */ }
  el.addEventListener("change", () => {
    try { localStorage.setItem(`sromap.${key}`, el.value); } catch (_) {}
  });
}
for (const [slider, out] of [
  [ui.bakeAzimuth, ui.bakeAzimuthOut],
  [ui.bakeElevation, ui.bakeElevationOut],
  [ui.bakeSamples, ui.bakeSamplesOut],
  [ui.bakeSoftness, ui.bakeSoftnessOut]
]) {
  slider.addEventListener("input", () => { out.textContent = slider.value; });
  out.textContent = slider.value;
}
ui.brushToolMode.addEventListener("click", () => setToolMode("brush"));
ui.selectToolMode.addEventListener("click", () => setToolMode("select"));
ui.snapToTerrainBtn.addEventListener("click", snapSelectedToTerrain);
ui.resetObjectBtn.addEventListener("click", resetSelectionToOriginal);
ui.deleteObjectBtn.addEventListener("click", deleteSelectedObject);
ui.spawnFilter.addEventListener("input", () => {
  state.spawnFilterText = ui.spawnFilter.value;
  renderSpawnList();
});
ui.spawnSection.addEventListener("toggle", () => {
  if (ui.spawnSection.open) {
    if (!state.objectCatalog) fetchObjectCatalog();
    if (!state.customCatalog) fetchCustomCatalog();
  }
});
ui.importCustomObjectBtn.addEventListener("click", openImportCustomObjectDialog);
ui.importCustomObjectDialog.addEventListener("close", onImportCustomObjectClose);

bindAxisInputs("X", ui.objXSlider, ui.objXInput, v => setSelectionAxis("X", v));
bindAxisInputs("Y", ui.objYSlider, ui.objYInput, v => setSelectionAxis("Y", v));
bindAxisInputs("Z", ui.objZSlider, ui.objZInput, v => setSelectionAxis("Z", v));
bindAxisInputs("Yaw", ui.objYawSlider, ui.objYawInput, v => setSelectionYawDegrees(v));

for (const btn of document.querySelectorAll(".rot-buttons button[data-rot]")) {
  btn.addEventListener("click", () => addSelectionYawDegrees(parseFloat(btn.dataset.rot)));
}

setupObjectPanelDrag();
restoreObjectPanelPosition();

updateBrushLabels();
updateSelectionUI();

window.addEventListener("keydown", e => {
  if (isTyping(e.target)) return;
  if ((e.ctrlKey || e.metaKey) && !e.altKey) {
    const k = (e.key || "").toLowerCase();
    if (k === "z" && !e.shiftKey) {
      e.preventDefault();
      doUndo();
      return;
    }
    if (k === "y" || (k === "z" && e.shiftKey)) {
      e.preventDefault();
      doRedo();
      return;
    }
  }
  state.keys.add(e.code);
  if (state.toolMode === "select" && state.selection) {
    // Suppress browser default for page-navigation keys while moving a selection.
    if (e.code === "ArrowUp" || e.code === "ArrowDown"
      || e.code === "ArrowLeft" || e.code === "ArrowRight"
      || e.code === "PageUp" || e.code === "PageDown") {
      e.preventDefault();
    }
  }
  if (e.code === "Escape" && state.selection) {
    state.selection = null;
    updateSelectionUI();
  }
  if (e.code === "KeyN" && !e.ctrlKey && !e.metaKey && !e.altKey) {
    state.showNVMOverlay = !state.showNVMOverlay;
    ui.showNVMOverlay.checked = state.showNVMOverlay;
    setStatus(state.showNVMOverlay ? "NVM overlay on" : "NVM overlay off");
  }
  if ((e.key === "m" || e.key === "M") && !e.ctrlKey && !e.metaKey && !e.altKey) {
    toggleWorldMap();
  }
  if (state.worldMapOpen) {
    const k = (e.key || "").toLowerCase();
    if ((e.ctrlKey || e.metaKey) && k === "c") {
      e.preventDefault();
      copyWorldMapSelection();
    }
    if ((e.ctrlKey || e.metaKey) && k === "v") {
      e.preventDefault();
      pasteAtHover();
    }
    if (e.code === "Escape") {
      if (state.worldMapClipboard) {
        clearWorldMapClipboard();
      } else {
        toggleWorldMap(false);
      }
    }
  }
  if ((e.code === "Delete" || e.code === "Backspace") && state.toolMode === "select" && state.selection) {
    deleteSelectedObject();
    e.preventDefault();
  }
});
window.addEventListener("keyup", e => state.keys.delete(e.code));
window.addEventListener("resize", resize);

canvas.addEventListener("contextmenu", e => e.preventDefault());
canvas.addEventListener("mousedown", e => {
  if (e.button === 0) {
    state.leftDown = true;
    if (state.toolMode === "select") {
      tryPickObject();
      beginObjectDrag();
      if (state.dragging && state.selection) {
        beginEditSession(state.objectPlacements[state.selection.index]._cid, "Move object");
      }
    } else {
      beginBrushStroke(state.brushMode);
      stampBrush(true);
    }
  }
  if (e.button === 1) {
    // Middle-click in equalize mode samples the terrain under cursor.
    if (state.toolMode === "brush" && state.brushMode === "equalize" && state.hover) {
      e.preventDefault();
      state.equalizeTarget = state.hover.y;
      refreshEqualizeStatus();
      setStatus(`Equalize target set to y=${state.equalizeTarget.toFixed(1)}`);
    }
  }
  if (e.button === 2) {
    state.rightDown = true;
    canvas.requestPointerLock?.();
  }
});
canvas.addEventListener("wheel", e => {
  // Brush radius adjustment with mouse wheel while a brush is active.
  if (state.toolMode !== "brush") return;
  if (state.worldMapOpen) return; // world map has its own wheel handler
  e.preventDefault();
  const slider = ui.brushRadius;
  const step = parseFloat(slider.step) || 10;
  const min = parseFloat(slider.min);
  const max = parseFloat(slider.max);
  const direction = e.deltaY < 0 ? 1 : -1;
  const next = clamp(parseFloat(slider.value) + direction * step, min, max);
  if (next !== parseFloat(slider.value)) {
    slider.value = String(next);
    slider.dispatchEvent(new Event("input", { bubbles: true }));
  }
}, { passive: false });
window.addEventListener("mouseup", e => {
  if (e.button === 0) {
    state.leftDown = false;
    const wasDragging = state.dragging !== null;
    state.dragging = null;
    if (wasDragging) endEditSession();
    endBrushStroke();
  }
  if (e.button === 2) {
    state.rightDown = false;
    if (document.pointerLockElement === canvas) {
      document.exitPointerLock?.();
    }
  }
});

// Recover from "stuck button" situations. If the user right-drags outside
// the browser window and releases the button there, the OS swallows the
// mouseup and state.rightDown stays true — every subsequent mouse move
// keeps rotating the camera. Clearing on blur / visibilitychange / mouse
// leaving the document gives us reliable escape hatches.
function clearMouseHeldState() {
  const wasDragging = state.dragging !== null;
  state.leftDown = false;
  state.rightDown = false;
  state.dragging = null;
  if (wasDragging) endEditSession();
  endBrushStroke();
  if (document.pointerLockElement === canvas) {
    document.exitPointerLock?.();
  }
}
window.addEventListener("blur", clearMouseHeldState);
document.addEventListener("visibilitychange", () => {
  if (document.hidden) clearMouseHeldState();
});
document.addEventListener("mouseleave", e => {
  // Only fires for the top-level document; ignore element-level leaves.
  if (e.relatedTarget === null && e.target === document) clearMouseHeldState();
});
window.addEventListener("pointercancel", clearMouseHeldState);
window.addEventListener("mousemove", e => {
  const rect = canvas.getBoundingClientRect();
  state.mouse.x = e.clientX - rect.left;
  state.mouse.y = e.clientY - rect.top;
  // Reconcile our held-button state with what the browser actually reports.
  // Right-click in particular can swallow the mouseup event (the contextmenu
  // pipeline takes priority even when preventDefault'd), so we use the
  // `buttons` bitmask each frame as the source of truth.
  const buttons = e.buttons || 0;
  if (state.rightDown && (buttons & 2) === 0) state.rightDown = false;
  if (state.leftDown && (buttons & 1) === 0) {
    state.leftDown = false;
    const wasDragging = state.dragging !== null;
    state.dragging = null;
    if (wasDragging) endEditSession();
    endBrushStroke();
  }
  if (state.rightDown) {
    state.camera.yaw -= e.movementX * 0.003;
    state.camera.pitch = clamp(state.camera.pitch - e.movementY * 0.003, -1.35, 1.25);
  }
  updateObjectDrag();
});

init().catch(err => setStatus(err.message));
requestAnimationFrame(frame);

async function init() {
  resize();
  const info = await fetchJSON("/api/info");
  state.info = info;
  ui.activeCount.textContent = String(info.activeCount);
  if (ui.exportDir) ui.exportDir.textContent = info.exportDir || "(disabled)";
  ui.regionX.value = info.bounds.centerX;
  ui.regionY.value = info.bounds.centerY;
  await loadSelectedRegion();
}

async function loadSelectedRegion() {
  const x = parseInt(ui.regionX.value, 10);
  const y = parseInt(ui.regionY.value, 10);
  const radius = clamp(parseInt(ui.loadRadius.value, 10) || 0, 0, 2);
  state.baseX = x;
  state.baseY = y;
  for (const region of state.regions.values()) {
    if (region.texture) gl.deleteTexture(region.texture);
    if (region.lightmap) gl.deleteTexture(region.lightmap);
    if (region.mesh) {
      gl.deleteBuffer(region.mesh.position);
      gl.deleteBuffer(region.mesh.color);
      if (region.mesh.normal) gl.deleteBuffer(region.mesh.normal);
      if (region.mesh.texCoord) gl.deleteBuffer(region.mesh.texCoord);
    }
  }
  state.regions.clear();
  state.dirty.clear();
  setStatus(`Loading ${x},${y} radius ${radius}`);

  const requests = [];
  for (let dz = -radius; dz <= radius; dz++) {
    for (let dx = -radius; dx <= radius; dx++) {
      const rx = x + dx;
      const ry = y + dz;
      if (rx < 0 || rx > 255 || ry < 0 || ry > 127) continue;
      requests.push(loadRegion(rx, ry));
    }
  }
  await Promise.all(requests);
  rebuildObjects();
  const centerHeight = heightAtWorld(0, 0)?.height || 0;
  state.camera.x = 0;
  state.camera.y = centerHeight + 650;
  state.camera.z = 2200;
  state.camera.yaw = Math.PI;
  state.camera.pitch = -0.35;
  updateMeta();
  setStatus("Left paint, right mouse look, WASD fly, Space/Shift vertical, R = sprint, Ctrl+Z = undo");
}

async function loadRegion(x, y, opts = {}) {
  const region = await fetchJSON(`/api/region?x=${x}&y=${y}`);
  if (!region.hasMesh || !region.heights || region.heights.length !== GRID_SIZE * GRID_SIZE) return null;
  // Pull in any custom-object placements stored alongside (sidecar file).
  // Custom IDs (>= 0x80000000) would corrupt .o2 if persisted there, so they
  // live in <root>/CustomObjects/placements/<y>/<x>.json instead.
  try {
    const custom = await fetchJSON(`/api/region/custom-placements?x=${x}&y=${y}`);
    const regionID = ((y & 0xff) << 8) | (x & 0xff);
    region.objects = region.objects || [];
    for (const c of custom.placements || []) {
      region.objects.push({
        objID: c.objID, uid: c.uid, regionID,
        regionX: x, regionY: y,
        x: c.x, y: c.y, z: c.z, yaw: c.yaw,
        big: false, isCpd: false, isCustom: true,
        objectPath: null
      });
    }
  } catch (_) { /* sidecar absent → no custom placements for this region */ }
  const record = {
    x, y,
    active: region.active,
    refRegion: region.refRegion || null,
    heights: Float32Array.from(region.heights),
    tileIDs: region.textureIDs ? Uint16Array.from(region.textureIDs) : new Uint16Array(GRID_SIZE * GRID_SIZE),
    objects: region.objects || [],
    minHeight: region.stats.minHeight,
    maxHeight: region.stats.maxHeight,
    mesh: null,
    texture: null,
    lightmap: null,
    textureURL: region.textureUrl || null
  };
  buildTerrainMesh(record);
  state.regions.set(keyFor(x, y), record);
  if (record.textureURL) loadRegionTexture(record);
  loadRegionLightmap(record);
  loadRegionNVMCells(record);
  if (opts.streaming) {
    appendRegionPlacements(record);
    updateMeta();
  }
  return record;
}

// Returns the region the camera is currently above (using the same coordinate
// math as heightAtWorld). Falls back to {baseX, baseY} when the camera is
// outside any loaded region — note: still valid input for streamLoadTick,
// since regions are inferred from world XZ alone.
function cameraRegion() {
  const cx = state.camera.x;
  const cz = state.camera.z;
  return {
    x: state.baseX - Math.round(cx / REGION_SIZE),
    y: state.baseY + Math.round(cz / REGION_SIZE)
  };
}

function streamLoadTick() {
  if (!state.streaming || state.regions.size === 0) return;
  const cur = cameraRegion();
  const last = state.lastCamRegion;
  if (last && last.x === cur.x && last.y === cur.y) return;
  state.lastCamRegion = cur;
  const radius = clamp(parseInt(ui.loadRadius.value, 10) || 0, 0, 4);
  for (let dz = -radius; dz <= radius; dz++) {
    for (let dx = -radius; dx <= radius; dx++) {
      const rx = cur.x + dx;
      const ry = cur.y + dz;
      if (rx < 0 || rx > 255 || ry < 0 || ry > 127) continue;
      const key = keyFor(rx, ry);
      if (state.regions.has(key) || state.streamingInFlight.has(key)) continue;
      state.streamingInFlight.add(key);
      loadRegion(rx, ry, { streaming: true })
        .catch(err => console.warn(`stream ${rx},${ry}:`, err))
        .finally(() => state.streamingInFlight.delete(key));
    }
  }
}

async function loadRegionNVMCells(region) {
  try {
    const data = await fetch(`/api/region/nvm-cells?x=${region.x}&y=${region.y}`);
    if (!data.ok) return;
    const json = await data.json();
    region.nvmCells = json.cells || [];
    region.nvmObjects = json.objects || [];
    region.nvmOpenCount = json.openCount || 0;
    buildNVMOverlayBuffers(region);
  } catch (_) { /* no NVM */ }
}

function buildNVMOverlayBuffers(region) {
  const cells = region.nvmCells || [];
  if (cells.length === 0) return;
  const offX = regionOffsetX(region.x);
  const offZ = regionOffsetZ(region.y);
  const openVerts = [];
  const closedVerts = [];
  for (let i = 0; i < cells.length; i++) {
    const c = cells[i];
    const w0X = offX + 960 - c.maxX;
    const w1X = offX + 960 - c.minX;
    const w0Z = offZ - 960 + c.minZ;
    const w1Z = offZ - 960 + c.maxZ;
    const corners = [[w0X, w0Z], [w1X, w0Z], [w1X, w1Z], [w0X, w1Z]];
    const ys = corners.map(([x, z]) => {
      const h = heightAtWorld(x, z);
      return (h ? h.height : 0) + 2;
    });
    const verts = c.open ? openVerts : closedVerts;
    for (let j = 0; j < 4; j++) {
      const k = (j + 1) % 4;
      verts.push(corners[j][0], ys[j], corners[j][1]);
      verts.push(corners[k][0], ys[k], corners[k][1]);
    }
  }
  if (region.nvmOpenBuffer) gl.deleteBuffer(region.nvmOpenBuffer);
  if (region.nvmClosedBuffer) gl.deleteBuffer(region.nvmClosedBuffer);
  region.nvmOpenBuffer = null;
  region.nvmClosedBuffer = null;
  region.nvmOpenVerts = 0;
  region.nvmClosedVerts = 0;
  if (openVerts.length > 0) {
    region.nvmOpenBuffer = makeBuffer(new Float32Array(openVerts));
    region.nvmOpenVerts = openVerts.length / 3;
  }
  if (closedVerts.length > 0) {
    region.nvmClosedBuffer = makeBuffer(new Float32Array(closedVerts));
    region.nvmClosedVerts = closedVerts.length / 3;
  }
}

function loadRegionLightmap(region, bust) {
  const cacheBust = bust ? `&v=${Date.now()}` : "";
  const url = `/api/region/lightmap?x=${region.x}&y=${region.y}${cacheBust}`;
  const key = keyFor(region.x, region.y);
  const img = new Image();
  img.onload = () => {
    const current = state.regions.get(key);
    if (current !== region) return;
    const tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    if (region.lightmap) gl.deleteTexture(region.lightmap);
    region.lightmap = tex;
  };
  img.onerror = () => { /* no lightmap available */ };
  img.src = url;
}

function loadRegionTexture(region) {
  const url = region.textureURL;
  if (!url) return;
  const key = keyFor(region.x, region.y);
  const img = new Image();
  img.onload = () => {
    const current = state.regions.get(key);
    if (current !== region) return;
    const tex = gl.createTexture();
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    if (region.texture) gl.deleteTexture(region.texture);
    region.texture = tex;
  };
  img.onerror = () => {
    // texture load failed; fallback shading stays
  };
  img.src = url;
}

function buildTerrainMesh(region) {
  const count = GRID_SIZE * GRID_SIZE;
  const positions = new Float32Array(count * 3);
  const colors = new Float32Array(count * 3);
  const normals = new Float32Array(count * 3);
  const texCoords = new Float32Array(count * 2);
  const offX = regionOffsetX(region.x);
  const offZ = regionOffsetZ(region.y);
  const minH = Math.min(...region.heights);
  const maxH = Math.max(...region.heights);
  const range = Math.max(1, maxH - minH);
  const uvDenom = GRID_SIZE - 1;

  for (let gz = 0; gz < GRID_SIZE; gz++) {
    for (let gx = 0; gx < GRID_SIZE; gx++) {
      const i = gz * GRID_SIZE + gx;
      const h = region.heights[i];
      const p = i * 3;
      positions[p] = offX + 960 - gx * CELL_SIZE;
      positions[p + 1] = h;
      positions[p + 2] = offZ - 960 + gz * CELL_SIZE;

      const t = clamp((h - minH) / range, 0, 1);
      const shade = region.active ? 1 : 0.65;
      colors[p] = shade * (0.12 + 0.26 * t);
      colors[p + 1] = shade * (0.34 + 0.24 * (1 - Math.abs(t - 0.45)));
      colors[p + 2] = shade * (0.13 + 0.13 * t);

      const u = i * 2;
      texCoords[u] = gx / uvDenom;
      texCoords[u + 1] = gz / uvDenom;
    }
  }

  for (let gz = 0; gz < GRID_SIZE; gz++) {
    for (let gx = 0; gx < GRID_SIZE; gx++) {
      const i = gz * GRID_SIZE + gx;
      const gxL = gx > 0 ? gx - 1 : gx;
      const gxR = gx < GRID_SIZE - 1 ? gx + 1 : gx;
      const gzD = gz > 0 ? gz - 1 : gz;
      const gzU = gz < GRID_SIZE - 1 ? gz + 1 : gz;
      const dhx = region.heights[gz * GRID_SIZE + gxR] - region.heights[gz * GRID_SIZE + gxL];
      const dhz = region.heights[gzU * GRID_SIZE + gx] - region.heights[gzD * GRID_SIZE + gx];
      // worldX decreases with gx, so flip sign on the x slope to keep the normal pointing up
      const nx = dhx;
      const ny = 2 * CELL_SIZE;
      const nz = -dhz;
      const len = Math.hypot(nx, ny, nz) || 1;
      const p = i * 3;
      normals[p] = nx / len;
      normals[p + 1] = ny / len;
      normals[p + 2] = nz / len;
    }
  }

  const position = makeBuffer(positions);
  const color = makeBuffer(colors);
  const normal = makeBuffer(normals);
  const texCoord = makeBuffer(texCoords);
  if (region.mesh) {
    gl.deleteBuffer(region.mesh.position);
    gl.deleteBuffer(region.mesh.color);
    if (region.mesh.normal) gl.deleteBuffer(region.mesh.normal);
    if (region.mesh.texCoord) gl.deleteBuffer(region.mesh.texCoord);
  }
  region.mesh = { position, color, normal, texCoord, count: terrainIndexCount };
}

function rebuildObjects() {
  state.objectPlacements = [];
  state.undoStack = [];
  state.redoStack = [];
  state.editSession = null;
  state.brushStroke = null;
  state.selection = null;
  state.dirtyPlacements.clear();
  state.pendingDeletes = [];
  const seen = new Set();
  for (const region of state.regions.values()) {
    appendRegionPlacements(region, seen);
  }
  rebuildObjectMarkers();
  refreshSaveDirtyState();
  updateSelectionUI();
  updateMeta();
}

// appendRegionPlacements adds the given region's objects to the global
// placement list without disturbing existing entries, undo stack, or dirty
// sets. Used by both the full rebuild path and the streaming loader.
function appendRegionPlacements(region, seen) {
  if (!seen) {
    seen = new Set();
    for (const p of state.objectPlacements) {
      seen.add(`${p.regionID}_${p.uid}_${p.objID}`);
    }
  }
  const wantedIDs = new Set();
  for (const obj of region.objects || []) {
    const key = `${obj.regionID}_${obj.uid}_${obj.objID}`;
    if (seen.has(key)) continue;
    seen.add(key);
    const offX = regionOffsetX(obj.regionX);
    const offZ = regionOffsetZ(obj.regionY);
    const wx = offX + 960 - obj.x;
    const wy = obj.y;
    const wz = offZ - 960 + obj.z;
    state.objectPlacements.push({
      _cid: state.nextClientID++,
      objID: obj.objID,
      uid: obj.uid,
      regionID: obj.regionID,
      regionX: obj.regionX,
      regionY: obj.regionY,
      // Snapshot of where this entry lives in the on-disk .o2 — used by
      // save to know whether the placement crossed a region boundary
      // since it was loaded (which needs delete+add instead of just edit).
      originalRegionID: obj.regionID,
      originalUID: obj.uid,
      localX: obj.x,
      localY: obj.y,
      localZ: obj.z,
      originalX: obj.x,
      originalY: obj.y,
      originalZ: obj.z,
      originalYaw: obj.yaw,
      wx, wy, wz,
      yaw: obj.yaw,
      big: obj.big,
      isCpd: obj.isCpd,
      isCustom: !!obj.isCustom,
      objectPath: obj.objectPath || ""
    });
    wantedIDs.add(obj.objID);
  }
  for (const id of wantedIDs) {
    if (!state.objectAssets.has(id)) {
      state.objectAssets.set(id, { state: "loading", meshes: null });
      fetchObjectAsset(id);
    }
  }
}

function rebuildObjectMarkers() {
  const positions = [];
  const colors = [];
  for (const p of state.objectPlacements) {
    positions.push(p.wx, p.wy + 25, p.wz);
    if (p.isCpd) colors.push(0.93, 0.61, 0.25);
    else if (p.big) colors.push(0.96, 0.82, 0.22);
    else colors.push(0.25, 0.78, 0.95);
  }
  state.objects.count = state.objectPlacements.length;
  state.objects.position = new Float32Array(positions);
  state.objects.color = new Float32Array(colors);
  gl.bindBuffer(gl.ARRAY_BUFFER, objectPositionBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, state.objects.position, gl.DYNAMIC_DRAW);
  gl.bindBuffer(gl.ARRAY_BUFFER, objectColorBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, state.objects.color, gl.DYNAMIC_DRAW);
}

async function fetchObjectAsset(id) {
  // High-ID range = user-imported OBJs served by the custom-object pipeline.
  const endpoint = id >= 0x80000000 ? `/api/custom-object?id=${id}` : `/api/object?id=${id}`;
  try {
    const data = await fetchJSON(endpoint);
    const meshes = (data.meshes || []).map(buildObjectMesh).filter(Boolean);
    state.objectAssets.set(id, {
      state: meshes.length ? "ready" : "empty",
      meshes,
      bboxMin: data.bboxMin || [0, 0, 0],
      bboxMax: data.bboxMax || [0, 0, 0],
      hasCollision: !!data.hasCollision,
      collisionBBoxMin: data.collisionBBoxMin || data.bboxMin || [0, 0, 0],
      collisionBBoxMax: data.collisionBBoxMax || data.bboxMax || [0, 0, 0],
      collisionNavVertices: data.collisionNavVertices || null,
      collisionNavIndices: data.collisionNavIndices || null,
      collisionNavOutlineIndices: data.collisionNavOutlineIndices || null,
      isCustom: !!data.isCustom
    });
  } catch (err) {
    state.objectAssets.set(id, { state: "error", meshes: null });
  }
}

function buildObjectMesh(mesh) {
  if (!mesh.vertices || !mesh.indices || mesh.indices.length === 0) return null;
  const vbuf = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, vbuf);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(mesh.vertices), gl.STATIC_DRAW);
  const ibuf = gl.createBuffer();
  gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, ibuf);
  gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, new Uint16Array(mesh.indices), gl.STATIC_DRAW);
  const record = { vbuf, ibuf, count: mesh.indices.length, texture: null };
  if (mesh.textureUrl) {
    const img = new Image();
    img.onload = () => {
      const tex = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, tex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, img);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
      record.texture = tex;
    };
    img.onerror = () => {};
    img.src = mesh.textureUrl;
  }
  return record;
}

function makeObjectMatrix(wx, wy, wz, yaw, out) {
  const c = Math.cos(yaw || 0);
  const s = Math.sin(yaw || 0);
  const m = out || new Float32Array(16);
  m[0] = -c;  m[1] = 0;  m[2] =  s; m[3] = 0;
  m[4] =  0;  m[5] = 1;  m[6] =  0; m[7] = 0;
  m[8] =  s;  m[9] = 0;  m[10] = c; m[11] = 0;
  m[12] = wx; m[13] = wy; m[14] = wz; m[15] = 1;
  return m;
}

function placementAssetReady(placement) {
  const asset = state.objectAssets.get(placement.objID);
  if (!asset || asset.state !== "ready") return null;
  return asset;
}

function placementWorldAABB(placement, fallbackRadius) {
  const asset = placementAssetReady(placement);
  let lmin, lmax;
  if (asset) {
    lmin = asset.bboxMin;
    lmax = asset.bboxMax;
  } else {
    const r = fallbackRadius || 60;
    lmin = [-r, 0, -r];
    lmax = [r, 2 * r, r];
  }
  const c = Math.cos(placement.yaw || 0);
  const s = Math.sin(placement.yaw || 0);
  let minX = Infinity, minY = Infinity, minZ = Infinity;
  let maxX = -Infinity, maxY = -Infinity, maxZ = -Infinity;
  for (let i = 0; i < 8; i++) {
    const x = (i & 1) ? lmax[0] : lmin[0];
    const y = (i & 2) ? lmax[1] : lmin[1];
    const z = (i & 4) ? lmax[2] : lmin[2];
    // model matrix = T * R_y * S(-1, 1, 1) applied to (x, y, z)
    const wx = -c * x + s * z + placement.wx;
    const wy = y + placement.wy;
    const wz = s * x + c * z + placement.wz;
    if (wx < minX) minX = wx;
    if (wx > maxX) maxX = wx;
    if (wy < minY) minY = wy;
    if (wy > maxY) maxY = wy;
    if (wz < minZ) minZ = wz;
    if (wz > maxZ) maxZ = wz;
  }
  return [minX, minY, minZ, maxX, maxY, maxZ];
}

function rayAABB(ro, rd, aabb) {
  let tmin = -Infinity, tmax = Infinity;
  for (let i = 0; i < 3; i++) {
    const inv = 1 / rd[i];
    let t1 = (aabb[i] - ro[i]) * inv;
    let t2 = (aabb[i + 3] - ro[i]) * inv;
    if (t1 > t2) { const tmp = t1; t1 = t2; t2 = tmp; }
    if (t1 > tmin) tmin = t1;
    if (t2 < tmax) tmax = t2;
    if (tmin > tmax) return -1;
  }
  if (tmax < 0) return -1;
  return tmin > 0 ? tmin : tmax;
}

function tryPickObject() {
  if (state.objectPlacements.length === 0) return;
  const ray = mouseRay();
  const ro = [state.camera.x, state.camera.y, state.camera.z];
  let bestIdx = -1;
  let bestT = Infinity;
  for (let i = 0; i < state.objectPlacements.length; i++) {
    const p = state.objectPlacements[i];
    const aabb = placementWorldAABB(p);
    const t = rayAABB(ro, ray, aabb);
    if (t > 0 && t < bestT) {
      bestT = t;
      bestIdx = i;
    }
  }
  if (bestIdx >= 0) {
    state.selection = { index: bestIdx };
    setStatus(`Selected ${state.objectPlacements[bestIdx].objectPath || `id ${state.objectPlacements[bestIdx].objID}`}`);
  } else {
    state.selection = null;
    setStatus("Nothing under cursor");
  }
  updateSelectionUI();
}

function rayPlaneY(ro, rd, py) {
  if (Math.abs(rd[1]) < 1e-6) return null;
  const t = (py - ro[1]) / rd[1];
  if (t <= 0) return null;
  return [ro[0] + rd[0] * t, py, ro[2] + rd[2] * t];
}

function beginObjectDrag() {
  if (state.toolMode !== "select" || !state.selection) {
    state.dragging = null;
    return;
  }
  const p = state.objectPlacements[state.selection.index];
  const ray = mouseRay();
  const ro = [state.camera.x, state.camera.y, state.camera.z];
  const hit = rayPlaneY(ro, ray, p.wy);
  if (!hit) {
    state.dragging = null;
    return;
  }
  state.dragging = {
    index: state.selection.index,
    planeY: p.wy,
    offsetX: p.wx - hit[0],
    offsetZ: p.wz - hit[2]
  };
}

function updateObjectDrag() {
  if (!state.dragging) return;
  if (!state.leftDown || state.toolMode !== "select") {
    state.dragging = null;
    return;
  }
  const idx = state.dragging.index;
  if (state.selection && state.selection.index !== idx) {
    state.dragging = null;
    return;
  }
  const p = state.objectPlacements[idx];
  const ray = mouseRay();
  const ro = [state.camera.x, state.camera.y, state.camera.z];
  const hit = rayPlaneY(ro, ray, state.dragging.planeY);
  if (!hit) return;
  p.wx = hit[0] + state.dragging.offsetX;
  p.wz = hit[2] + state.dragging.offsetZ;
  recomputeLocalFromWorld(p);
  rebuildObjectMarkers();
  markPlacementDirty(idx);
  updateSelectionUI();
}

function recomputeLocalFromWorld(p) {
  // If the world position drifted out of the placement's home region (e.g.
  // a drag carried it across a region boundary), re-home to the region that
  // actually contains the cursor. Otherwise local coords go negative or
  // exceed 1920 and break NVM tile-flag lookups for collision.
  const newRegX = state.baseX - Math.floor((p.wx + 960) / REGION_SIZE);
  const newRegY = state.baseY + Math.floor((p.wz + 960) / REGION_SIZE);
  if (newRegX !== p.regionX || newRegY !== p.regionY) {
    p.regionX = newRegX;
    p.regionY = newRegY;
    p.regionID = ((newRegY & 0xff) << 8) | (newRegX & 0xff);
  }
  const offX = regionOffsetX(p.regionX);
  const offZ = regionOffsetZ(p.regionY);
  p.localX = offX + 960 - p.wx;
  p.localY = p.wy;
  p.localZ = p.wz - offZ + 960;
}

// --- Undo / redo --------------------------------------------------------

function pushUndo(cmd) {
  state.undoStack.push(cmd);
  if (state.undoStack.length > state.undoLimit) state.undoStack.shift();
  state.redoStack.length = 0;
  setStatus(`${cmd.label} (Ctrl+Z to undo)`);
}

function doUndo() {
  endEditSession();
  endBrushStroke();
  const cmd = state.undoStack.pop();
  if (!cmd) { setStatus("Nothing to undo"); return; }
  cmd.undo();
  state.redoStack.push(cmd);
  setStatus(`Undo: ${cmd.label}`);
}

function doRedo() {
  endEditSession();
  endBrushStroke();
  const cmd = state.redoStack.pop();
  if (!cmd) { setStatus("Nothing to redo"); return; }
  cmd.redo();
  state.undoStack.push(cmd);
  setStatus(`Redo: ${cmd.label}`);
}

function findPlacementByCid(cid) {
  for (let i = 0; i < state.objectPlacements.length; i++) {
    if (state.objectPlacements[i]._cid === cid) return i;
  }
  return -1;
}

function snapshotPlacement(p) {
  return {
    localX: p.localX,
    localY: p.localY,
    localZ: p.localZ,
    yaw: p.yaw
  };
}

function applyPlacementSnap(idx, snap) {
  if (idx < 0) return;
  const p = state.objectPlacements[idx];
  p.localX = snap.localX;
  p.localY = snap.localY;
  p.localZ = snap.localZ;
  p.yaw = snap.yaw;
  applyPlacementLocalToWorld(p);
  rebuildObjectMarkers();
  markPlacementDirty(idx);
  if (state.selection && state.selection.index === idx) updateSelectionUI();
}

function beginEditSession(cid, label) {
  if (state.editSession && state.editSession.cid === cid) return;
  endEditSession();
  const idx = findPlacementByCid(cid);
  if (idx < 0) return;
  state.editSession = {
    cid,
    label: label || "Edit object",
    before: snapshotPlacement(state.objectPlacements[idx])
  };
}

function endEditSession() {
  const sess = state.editSession;
  state.editSession = null;
  if (!sess) return;
  const idx = findPlacementByCid(sess.cid);
  if (idx < 0) return;
  const after = snapshotPlacement(state.objectPlacements[idx]);
  if (samePlacementSnap(sess.before, after)) return;
  const { cid, label, before } = sess;
  pushUndo({
    label,
    undo: () => applyPlacementSnap(findPlacementByCid(cid), before),
    redo: () => applyPlacementSnap(findPlacementByCid(cid), after)
  });
}

function samePlacementSnap(a, b) {
  return a.localX === b.localX && a.localY === b.localY && a.localZ === b.localZ && a.yaw === b.yaw;
}

function bumpEditIdle() {
  if (!state.editSession) return;
  clearTimeout(state.editIdleTimer);
  state.editIdleTimer = setTimeout(endEditSession, 500);
}

function beginBrushStroke(mode) {
  if (state.brushStroke) return;
  state.brushStroke = { mode, regionDeltas: new Map() };
}

function recordBrushVertex(regionKey, idx, oldValue) {
  if (!state.brushStroke) return;
  let m = state.brushStroke.regionDeltas.get(regionKey);
  if (!m) {
    m = new Map();
    state.brushStroke.regionDeltas.set(regionKey, m);
  }
  if (!m.has(idx)) m.set(idx, oldValue);
}

function endBrushStroke() {
  const stroke = state.brushStroke;
  state.brushStroke = null;
  if (!stroke || stroke.regionDeltas.size === 0) return;
  const mode = stroke.mode;
  const label = mode === "tile" ? "Tile paint" : (mode === "lower" ? "Lower terrain" : "Raise terrain");
  // Capture after-values now while regions are in scope.
  const final = new Map();
  for (const [regionKey, before] of stroke.regionDeltas) {
    const region = state.regions.get(regionKey);
    if (!region) continue;
    const triples = [];
    for (const [idx, oldVal] of before) {
      const newVal = mode === "tile" ? region.tileIDs[idx] : region.heights[idx];
      if (newVal !== oldVal) triples.push([idx, oldVal, newVal]);
    }
    if (triples.length > 0) final.set(regionKey, triples);
  }
  if (final.size === 0) return;
  pushUndo({
    label,
    undo: () => applyBrushStroke(final, mode, 1),
    redo: () => applyBrushStroke(final, mode, 2)
  });
}

function applyBrushStroke(final, mode, sourceCol) {
  // sourceCol: 1 = oldVal, 2 = newVal
  for (const [regionKey, triples] of final) {
    const region = state.regions.get(regionKey);
    if (!region) continue;
    if (mode === "tile") {
      for (const t of triples) region.tileIDs[t[0]] = t[sourceCol];
      state.dirty.add(regionKey);
      state.tileDirty.add(regionKey);
    } else {
      for (const t of triples) region.heights[t[0]] = t[sourceCol];
      buildTerrainMesh(region);
      state.dirty.add(regionKey);
    }
  }
  updateMeta();
}

// --- end undo ----------------------------------------------------------

function updateSelectionUI() {
  const sel = state.selection;
  if (!sel) {
    ui.selObjName.textContent = "None";
    ui.selObjMeta.textContent = "-";
    ui.objectEdit.disabled = true;
    ui.resetObjectBtn.disabled = true;
    ui.snapToTerrainBtn.disabled = true;
    if (ui.deleteObjectBtn) ui.deleteObjectBtn.disabled = true;
    return;
  }
  ui.objectEdit.disabled = false;
  ui.resetObjectBtn.disabled = false;
  ui.snapToTerrainBtn.disabled = false;
  if (ui.deleteObjectBtn) ui.deleteObjectBtn.disabled = false;
  syncSelectionLabels();
  syncSelectionInputs();
}

function syncSelectionLabels() {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  ui.selObjName.textContent = p.objectPath || `objID ${p.objID}`;
  ui.selObjMeta.textContent = `id ${p.objID} · uid ${p.uid} · region ${p.regionX},${p.regionY}`;
}

function syncSelectionInputs() {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  setSliderPair(ui.objXSlider, ui.objXInput, p.localX);
  setSliderPair(ui.objYSlider, ui.objYInput, p.localY);
  setSliderPair(ui.objZSlider, ui.objZInput, p.localZ);
  setSliderPair(ui.objYawSlider, ui.objYawInput, normalizeAngleDeg(p.yaw * 180 / Math.PI));
}

function setSliderPair(slider, input, value) {
  slider.value = String(value);
  input.value = String(Math.round(value * 100) / 100);
}

function normalizeAngleDeg(deg) {
  let v = deg % 360;
  if (v > 180) v -= 360;
  if (v < -180) v += 360;
  return v;
}

function bindAxisInputs(name, slider, input, apply) {
  let lastValid = 0;
  slider.addEventListener("input", () => {
    if (state.selection) beginEditSession(state.objectPlacements[state.selection.index]._cid, `Edit ${name}`);
    const v = parseFloat(slider.value);
    if (Number.isFinite(v)) { lastValid = v; apply(v); }
  });
  slider.addEventListener("change", endEditSession);
  input.addEventListener("input", () => {
    if (state.selection) beginEditSession(state.objectPlacements[state.selection.index]._cid, `Edit ${name}`);
    const v = parseFloat(input.value);
    if (Number.isFinite(v)) { lastValid = v; apply(v); }
  });
  input.addEventListener("change", () => {
    if (!Number.isFinite(parseFloat(input.value))) {
      input.value = String(Math.round(lastValid * 100) / 100);
    }
    endEditSession();
  });
  input.addEventListener("blur", endEditSession);
}

function setSelectionAxis(axis, value) {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  if (axis === "X") p.localX = value;
  else if (axis === "Y") p.localY = value;
  else if (axis === "Z") p.localZ = value;
  applyPlacementLocalToWorld(p);
  rebuildObjectMarkers();
  markPlacementDirty(sel.index);
  syncSelectionLabels();
  // Sync the OTHER inputs (e.g. X slider while user is typing in X number input).
  // Skip syncing the input that's being interacted with to avoid clobbering.
  if (axis !== "X") setSliderPair(ui.objXSlider, ui.objXInput, p.localX);
  if (axis !== "Y") setSliderPair(ui.objYSlider, ui.objYInput, p.localY);
  if (axis !== "Z") setSliderPair(ui.objZSlider, ui.objZInput, p.localZ);
}

function setSelectionYawDegrees(deg) {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  p.yaw = deg * Math.PI / 180;
  rebuildObjectMarkers();
  markPlacementDirty(sel.index);
  syncSelectionLabels();
}

function addSelectionYawDegrees(deltaDeg) {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  beginEditSession(p._cid, `Rotate ${deltaDeg > 0 ? "+" : ""}${deltaDeg}°`);
  p.yaw += deltaDeg * Math.PI / 180;
  rebuildObjectMarkers();
  markPlacementDirty(sel.index);
  syncSelectionLabels();
  setSliderPair(ui.objYawSlider, ui.objYawInput, normalizeAngleDeg(p.yaw * 180 / Math.PI));
  endEditSession();
}

function resetSelectionToOriginal() {
  const sel = state.selection;
  if (!sel) return;
  const p = state.objectPlacements[sel.index];
  beginEditSession(p._cid, "Reset to original");
  p.localX = p.originalX;
  p.localY = p.originalY;
  p.localZ = p.originalZ;
  p.yaw = p.originalYaw;
  applyPlacementLocalToWorld(p);
  rebuildObjectMarkers();
  state.dirtyPlacements.delete(sel.index);
  refreshSaveDirtyState();
  updateSelectionUI();
  endEditSession();
  setStatus("Reset to original placement");
}

function setToolMode(mode) {
  state.toolMode = mode;
  ui.brushToolMode.classList.toggle("active", mode === "brush");
  ui.selectToolMode.classList.toggle("active", mode === "select");
  ui.brushControls.hidden = (mode !== "brush");
  ui.selectControls.hidden = (mode !== "select");
  ui.objectPanel.hidden = (mode !== "select");
  if (mode !== "select") {
    state.selection = null;
    updateSelectionUI();
  }
}

function setupObjectPanelDrag() {
  let drag = null;
  ui.objectPanelHeader.addEventListener("mousedown", e => {
    if (e.button !== 0) return;
    // Don't start dragging when clicking interactive elements inside the header.
    if (e.target.closest("button, input, select")) return;
    const rect = ui.objectPanel.getBoundingClientRect();
    drag = {
      offX: e.clientX - rect.left,
      offY: e.clientY - rect.top
    };
    ui.objectPanel.classList.add("dragging");
    e.preventDefault();
  });
  window.addEventListener("mousemove", e => {
    if (!drag) return;
    const rect = ui.objectPanel.getBoundingClientRect();
    const maxX = window.innerWidth - rect.width;
    const maxY = window.innerHeight - rect.height;
    const x = clamp(e.clientX - drag.offX, 0, Math.max(0, maxX));
    const y = clamp(e.clientY - drag.offY, 0, Math.max(0, maxY));
    ui.objectPanel.style.left = `${x}px`;
    ui.objectPanel.style.top = `${y}px`;
    ui.objectPanel.style.right = "auto";
  });
  window.addEventListener("mouseup", () => {
    if (!drag) return;
    drag = null;
    ui.objectPanel.classList.remove("dragging");
    saveObjectPanelPosition();
  });
}

function saveObjectPanelPosition() {
  try {
    const style = ui.objectPanel.style;
    if (!style.left || !style.top) return;
    localStorage.setItem("sromap.objectPanel", JSON.stringify({
      left: style.left,
      top: style.top
    }));
  } catch (_) { /* ignore */ }
}

function restoreObjectPanelPosition() {
  try {
    const raw = localStorage.getItem("sromap.objectPanel");
    if (!raw) return;
    const saved = JSON.parse(raw);
    if (!saved || typeof saved.left !== "string" || typeof saved.top !== "string") return;
    ui.objectPanel.style.left = saved.left;
    ui.objectPanel.style.top = saved.top;
    ui.objectPanel.style.right = "auto";
  } catch (_) { /* ignore */ }
}

function applyPlacementLocalToWorld(p) {
  const offX = regionOffsetX(p.regionX);
  const offZ = regionOffsetZ(p.regionY);
  p.wx = offX + 960 - p.localX;
  p.wy = p.localY;
  p.wz = offZ - 960 + p.localZ;
}

function markPlacementDirty(index) {
  state.dirtyPlacements.add(index);
  refreshSaveDirtyState();
}

function updateSelectedObject(dt) {
  if (state.toolMode !== "select" || !state.selection) return;
  const p = state.objectPlacements[state.selection.index];

  const fast = state.keys.has("ShiftLeft") || state.keys.has("ShiftRight");
  const slow = state.keys.has("ControlLeft") || state.keys.has("ControlRight");
  const baseSpeed = fast ? 500 : (slow ? 20 : 120);
  const yawSpeed = fast ? 1.6 : (slow ? 0.1 : 0.6);
  const step = baseSpeed * dt;

  // camera-relative XZ basis (flatten y)
  const basis = cameraBasis();
  const fwd = normalize([basis.forward[0], 0, basis.forward[2]]);
  const right = normalize([basis.right[0], 0, basis.right[2]]);

  let moved = false;
  if (state.keys.has("ArrowUp"))    { p.wx += fwd[0] * step;   p.wz += fwd[2] * step;   moved = true; }
  if (state.keys.has("ArrowDown"))  { p.wx -= fwd[0] * step;   p.wz -= fwd[2] * step;   moved = true; }
  if (state.keys.has("ArrowRight")) { p.wx += right[0] * step; p.wz += right[2] * step; moved = true; }
  if (state.keys.has("ArrowLeft"))  { p.wx -= right[0] * step; p.wz -= right[2] * step; moved = true; }
  if (state.keys.has("PageUp"))   { p.wy += step; moved = true; }
  if (state.keys.has("PageDown")) { p.wy -= step; moved = true; }
  if (state.keys.has("Comma"))  { p.yaw -= yawSpeed * dt; moved = true; }
  if (state.keys.has("Period")) { p.yaw += yawSpeed * dt; moved = true; }
  if (state.keys.has("Backspace") || state.keys.has("Delete")) {
    // not implemented; consumed
  }

  if (moved) {
    if (!state.editSession) beginEditSession(p._cid, "Move object (keys)");
    bumpEditIdle();
    recomputeLocalFromWorld(p);
    rebuildObjectMarkers();
    markPlacementDirty(state.selection.index);
    updateSelectionUI();
  }
}

function snapSelectedToTerrain() {
  if (!state.selection) return;
  const p = state.objectPlacements[state.selection.index];
  const h = heightAtWorld(p.wx, p.wz);
  if (!h) {
    setStatus("No terrain under selection");
    return;
  }
  beginEditSession(p._cid, "Snap to terrain");
  p.wy = h.height;
  p.localY = h.height;
  rebuildObjectMarkers();
  markPlacementDirty(state.selection.index);
  updateSelectionUI();
  endEditSession();
  setStatus(`Snapped to y=${h.height.toFixed(1)}`);
}

function hasUnsavedObjectChanges() {
  if (state.dirtyPlacements.size > 0) return true;
  if (state.pendingDeletes.length > 0) return true;
  for (const p of state.objectPlacements) if (p.isNew) return true;
  return false;
}

function refreshSaveDirtyState() {
  const dirty = hasUnsavedObjectChanges();
  if (ui.objectSaveStatus) {
    const n = state.dirtyPlacements.size;
    const nDel = state.pendingDeletes.length;
    const nAdd = state.objectPlacements.filter(p => p.isNew).length;
    if (!dirty) ui.objectSaveStatus.textContent = "";
    else ui.objectSaveStatus.textContent = `${n} edits · ${nDel} deletes · ${nAdd} adds (use Save above)`;
  }
  refreshSaveAllButton();
}

function refreshSaveAllButton() {
  if (!ui.saveAllBtn) return;
  if (state.saveJustFlashed) return; // flashSavedJustNow is in control of the label
  const regions = state.dirty.size;
  const objEdits = state.dirtyPlacements.size;
  const objDeletes = state.pendingDeletes.length;
  const objAdds = state.objectPlacements.filter(p => p.isNew).length;
  const objTotal = objEdits + objDeletes + objAdds;
  const anyDirty = regions > 0 || objTotal > 0;
  ui.saveAllBtn.classList.toggle("dirty", anyDirty);
  ui.saveAllBtn.disabled = !anyDirty || state.saving === true;
  if (!anyDirty) {
    ui.saveAllBtn.textContent = "Saved";
    return;
  }
  const parts = [];
  if (regions > 0) parts.push(`${regions} region${regions === 1 ? "" : "s"}`);
  if (objTotal > 0) parts.push(`${objTotal} object${objTotal === 1 ? "" : "s"}`);
  ui.saveAllBtn.textContent = `Save + NVM · ${parts.join(" · ")}`;
}

// flashSavedJustNow gives the Save button a brief green "✓ Saved" state to
// confirm a server-side direct-write action (paste, create, delete) actually
// landed on disk. Without this the Save button stays at "Saved" with no
// indication that anything happened, which is confusing right after a paste.
function flashSavedJustNow(label) {
  if (!ui.saveAllBtn) return;
  clearTimeout(state._saveFlashTimer);
  state.saveJustFlashed = true;
  ui.saveAllBtn.classList.remove("dirty");
  ui.saveAllBtn.classList.add("just-saved");
  ui.saveAllBtn.disabled = true;
  ui.saveAllBtn.textContent = label || "✓ Saved";
  state._saveFlashTimer = setTimeout(() => {
    state.saveJustFlashed = false;
    ui.saveAllBtn.classList.remove("just-saved");
    refreshSaveAllButton();
  }, 2500);
}

async function saveAll(opts = {}) {
  if (state.saving) return;
  state.saving = true;
  refreshSaveAllButton();
  const rebuildTargets = new Map();
  try {
    if (hasUnsavedCustomPlacements()) {
      addRegionTargets(rebuildTargets, await saveCustomPlacements({ silent: true }));
    }
    if (hasUnsavedObjectChanges()) {
      addRegionTargets(rebuildTargets, await saveObjectEdits({ silent: true }));
    }
    if (state.dirty.size > 0) {
      addRegionTargets(rebuildTargets, await saveDirtyRegions({ silent: true }));
    }
    if (opts.rebuildNVM !== false && rebuildTargets.size > 0) {
      const regions = expandNVMRebuildTargets([...rebuildTargets.values()], { radius: 0 });
      const result = await rebuildMapOnlyNVMs(regions, {
        createMissing: true,
        statusPrefix: "Save NVM"
      });
      setStatus(`Saved and rebuilt ${result.done} NVM(s)`);
    } else {
      setStatus("All changes saved");
    }
  } catch (err) {
    setStatus(`Save/rebuild failed: ${err.message}`);
  } finally {
    state.saving = false;
    refreshSaveAllButton();
  }
}

function addRegionTargets(targets, regions) {
  if (!regions) return;
  for (const region of regions) {
    if (!region) continue;
    targets.set(keyFor(region.x, region.y), { x: region.x, y: region.y });
  }
}

function regionFromID(regionID) {
  return { x: regionID & 0xff, y: (regionID >> 8) & 0xff };
}

function expandNVMRebuildTargets(regions, opts = {}) {
  const radius = opts.radius ?? 1;
  const out = new Map();
  for (const region of regions) {
    for (let dy = -radius; dy <= radius; dy++) {
      for (let dx = -radius; dx <= radius; dx++) {
        const x = region.x + dx;
        const y = region.y + dy;
        if (x < 0 || x > 255 || y < 0 || y > 127) continue;
        out.set(keyFor(x, y), { x, y });
      }
    }
  }
  return [...out.values()];
}

function hasUnsavedCustomPlacements() {
  for (const idx of state.dirtyPlacements) {
    if (state.objectPlacements[idx]?.isCustom) return true;
  }
  for (const d of state.pendingDeletes) {
    if (d.objID >= 0x80000000) return true;
  }
  for (const p of state.objectPlacements) {
    if (p.isNew && p.isCustom) return true;
  }
  return false;
}

// saveCustomPlacements writes the full custom-placement sidecar for every
// region with any custom-related change since the last save. Since each
// region's sidecar is the *complete* list (not a delta), we send the
// in-memory snapshot per affected region.
async function saveCustomPlacements(opts = {}) {
  const affected = new Set();
  for (const idx of state.dirtyPlacements) {
    const p = state.objectPlacements[idx];
    if (p?.isCustom) {
      affected.add(p.regionID);
      if (p.originalRegionID !== undefined) affected.add(p.originalRegionID);
    }
  }
  for (const d of state.pendingDeletes) {
    if (d.objID >= 0x80000000) affected.add(d.regionID);
  }
  for (const p of state.objectPlacements) {
    if (p.isNew && p.isCustom) affected.add(p.regionID);
  }
  if (affected.size === 0) return [];
  const byRegion = new Map();
  for (const p of state.objectPlacements) {
    if (!p.isCustom) continue;
    if (!affected.has(p.regionID)) continue;
    if (!byRegion.has(p.regionID)) byRegion.set(p.regionID, []);
    byRegion.get(p.regionID).push(p);
  }
  // Regions in `affected` with no live custom placements still need a save
  // (so the sidecar is cleaned to []), which the server handles by removing
  // the empty file.
  for (const regionID of affected) {
    if (!byRegion.has(regionID)) byRegion.set(regionID, []);
  }
  for (const [regionID, placements] of byRegion) {
    const rx = regionID & 0xff;
    const ry = (regionID >> 8) & 0xff;
    const body = {
      placements: placements.map(p => ({
        objID: p.objID,
        uid: p.uid > 0 ? p.uid : 0,
        x: p.localX, y: p.localY, z: p.localZ,
        yaw: p.yaw
      }))
    };
    const result = await fetchJSON(`/api/region/custom-placements?x=${rx}&y=${ry}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
    // Adopt server-assigned UIDs back into our in-memory copies.
    const saved = result.placements || [];
    let cursor = 0;
    for (const p of placements) {
      if (p.uid <= 0 && cursor < saved.length) {
        p.uid = saved[cursor].uid;
      }
      p.isNew = false;
      p.originalX = p.localX;
      p.originalY = p.localY;
      p.originalZ = p.localZ;
      p.originalYaw = p.yaw;
      p.originalRegionID = p.regionID;
      p.originalUID = p.uid;
      cursor++;
    }
  }
  // Drop pending custom deletes — they're now committed.
  state.pendingDeletes = state.pendingDeletes.filter(d => d.objID < 0x80000000);
  // Drop dirty marks that targeted custom placements.
  const next = new Set();
  for (const idx of state.dirtyPlacements) {
    const p = state.objectPlacements[idx];
    if (p && !p.isCustom) next.add(idx);
  }
  state.dirtyPlacements = next;
  refreshSaveDirtyState();
  if (!opts.silent) setStatus(`Saved ${byRegion.size} custom region(s)`);
  return [...affected].map(regionFromID);
}

function removePlacementAt(index) {
  state.objectPlacements.splice(index, 1);
  // Reindex dirty set since array indices shifted
  const next = new Set();
  for (const idx of state.dirtyPlacements) {
    if (idx < index) next.add(idx);
    else if (idx > index) next.add(idx - 1);
  }
  state.dirtyPlacements = next;
  if (state.selection && state.selection.index === index) {
    state.selection = null;
  } else if (state.selection && state.selection.index > index) {
    state.selection.index -= 1;
  }
}

function deleteSelectedObject() {
  if (!state.selection) return;
  const idx = state.selection.index;
  const p = state.objectPlacements[idx];
  const snapshot = JSON.parse(JSON.stringify(p));
  const insertAt = idx;
  if (!p.isNew) {
    state.pendingDeletes.push({
      objID: p.objID,
      uid: p.uid,
      regionID: p.regionID
    });
  }
  removePlacementAt(idx);
  rebuildObjectMarkers();
  updateSelectionUI();
  refreshSaveDirtyState();
  pushUndo({
    label: snapshot.isNew ? "Delete (spawned)" : "Delete object",
    undo: () => {
      state.objectPlacements.splice(insertAt, 0, { ...snapshot });
      if (!snapshot.isNew) {
        const matchIdx = state.pendingDeletes.findIndex(d =>
          d.objID === snapshot.objID && d.uid === snapshot.uid && d.regionID === snapshot.regionID);
        if (matchIdx >= 0) state.pendingDeletes.splice(matchIdx, 1);
      }
      rebuildObjectMarkers();
      refreshSaveDirtyState();
    },
    redo: () => {
      const i = findPlacementByCid(snapshot._cid);
      if (i < 0) return;
      if (!snapshot.isNew) {
        state.pendingDeletes.push({
          objID: snapshot.objID, uid: snapshot.uid, regionID: snapshot.regionID
        });
      }
      removePlacementAt(i);
      rebuildObjectMarkers();
      refreshSaveDirtyState();
    }
  });
  setStatus(`Removed ${p.objectPath || `objID ${p.objID}`}`);
}

async function fetchObjectCatalog() {
  ui.spawnStatus.textContent = "Loading catalog...";
  try {
    const data = await fetchJSON("/api/objects");
    state.objectCatalog = (data.entries || []).slice().sort((a, b) => {
      return a.path.localeCompare(b.path);
    });
    updateSpawnStatus();
    renderSpawnList();
  } catch (err) {
    ui.spawnStatus.textContent = `Failed: ${err.message}`;
  }
}

async function fetchCustomCatalog() {
  try {
    const data = await fetchJSON("/api/custom-objects");
    state.customCatalog = (data.entries || []).slice().sort((a, b) => a.id - b.id);
    updateSpawnStatus();
    renderSpawnList();
  } catch (err) {
    state.customCatalog = [];
  }
}

function updateSpawnStatus() {
  const game = state.objectCatalog ? state.objectCatalog.length : 0;
  const custom = state.customCatalog ? state.customCatalog.length : 0;
  ui.spawnStatus.textContent =
    `${game} game · ${custom} custom${custom > 0 ? "" : " (use Import…)"}`;
}

function renderSpawnList() {
  if (!state.objectCatalog && !state.customCatalog) return;
  const filter = state.spawnFilterText.trim().toLowerCase();
  ui.spawnList.innerHTML = "";

  // Custom objects first so the user's just-imported asset is easy to find.
  if (state.customCatalog && state.customCatalog.length > 0) {
    const customMatches = state.customCatalog.filter(e => {
      if (filter === "") return true;
      return e.name.toLowerCase().includes(filter) || String(e.id) === filter;
    });
    for (const e of customMatches) {
      const wrap = document.createElement("div");
      wrap.className = "spawn-row-wrap";
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "spawn-row custom";
      const exportedBadge = e.gameObjID
        ? `<span class="row-game-id">→ ${String(e.gameObjID).padStart(5, "0")}</span>`
        : "";
      btn.innerHTML = `<span class="row-id">CUST</span>${escapeHtml(e.name)}${exportedBadge}`;
      btn.addEventListener("click", () => {
        // After export, the custom object has a real game objID — spawn with
        // that so the placement lands in the region's .o2 (in-game visible)
        // instead of the editor-only sidecar.
        const useGameId = !!e.gameObjID;
        spawnObjectAtHover({
          id: useGameId ? e.gameObjID : e.id,
          path: e.name,
          isCpd: false,
          isCustom: !useGameId
        });
        if (useGameId) {
          // Custom assets are served by /api/custom-object using the high
          // custom ID; preload that under the game ID so the mesh renders
          // immediately in the editor instead of waiting for /api/object.
          if (!state.objectAssets.has(e.gameObjID)) {
            const ghost = state.objectAssets.get(e.id);
            if (ghost) {
              state.objectAssets.set(e.gameObjID, ghost);
            }
          }
        }
      });
      wrap.appendChild(btn);
      const exp = document.createElement("button");
      exp.type = "button";
      exp.className = "spawn-row-export";
      exp.title = e.gameObjID
        ? `Re-export game files (current id ${e.gameObjID})`
        : "Generate .bsr/.bms/.bmt/.ddj + register in object.ifo for in-game use";
      exp.textContent = e.gameObjID ? "↻" : "→ game";
      exp.addEventListener("click", (ev) => {
        ev.stopPropagation();
        exportCustomObject(e);
      });
      wrap.appendChild(exp);
      ui.spawnList.appendChild(wrap);
    }
    if (customMatches.length > 0 && state.objectCatalog) {
      const sep = document.createElement("div");
      sep.className = "spawn-sep";
      sep.textContent = "— game catalog —";
      ui.spawnList.appendChild(sep);
    }
  }

  const matches = [];
  if (state.objectCatalog) {
    for (const e of state.objectCatalog) {
      if (filter === "") matches.push(e);
      else if (e.path.includes(filter) || String(e.id) === filter) matches.push(e);
      if (matches.length >= 120) break;
    }
    for (const e of matches) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "spawn-row";
      btn.innerHTML = `<span class="row-id">${String(e.id).padStart(5, "0")}</span>${escapeHtml(e.path)}`;
      btn.addEventListener("click", () => spawnObjectAtHover(e));
      ui.spawnList.appendChild(btn);
    }
  }
  if (ui.spawnList.children.length === 0) {
    const div = document.createElement("div");
    div.className = "hint";
    div.style.padding = "6px 8px";
    div.textContent = "No matches";
    ui.spawnList.appendChild(div);
  }
}

async function exportCustomObject(entry) {
  try {
    setStatus(`Exporting "${entry.name}" to game files...`);
    const result = await fetchJSON(`/api/custom-object/export?id=${entry.id}`, { method: "POST" });
    setStatus(`Exported "${result.slug}" → game objID ${result.gameObjID} (.bsr/.bms/.bmt/.ddj written)`);
    // Refresh both catalogs so the new game entry shows up and the custom
    // badge reflects the assigned objID.
    await fetchCustomCatalog();
    await fetchObjectCatalog();
  } catch (err) {
    setStatus(`Export failed: ${err.message}`);
  }
}

function openImportCustomObjectDialog() {
  ui.importCustomFolder.value = "";
  ui.importCustomName.value = "";
  ui.importCustomSize.value = "300";
  ui.importCustomObjectDialog.showModal();
  setTimeout(() => ui.importCustomFolder.focus(), 0);
}

async function onImportCustomObjectClose() {
  const dlg = ui.importCustomObjectDialog;
  console.log("[import] dialog closed, returnValue =", dlg.returnValue);
  if (dlg.returnValue !== "confirm") return;
  // Strip optional surrounding quotes a user might paste in.
  const folder = ui.importCustomFolder.value.trim().replace(/^["']|["']$/g, "");
  console.log("[import] folder =", folder);
  if (!folder) { setStatus("Import cancelled: no folder given"); return; }
  const name = ui.importCustomName.value.trim();
  const size = parseFloat(ui.importCustomSize.value);
  try {
    setStatus(`Importing OBJ from ${folder}...`);
    console.log("[import] POST /api/custom-object/import");
    const result = await fetchJSON("/api/custom-object/import", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        sourceFolder: folder,
        name: name || undefined,
        targetSize: isFinite(size) && size > 0 ? size : undefined
      })
    });
    console.log("[import] server response:", result);
    setStatus(`Imported "${result.name}" (id ${result.id}) — pick it from the spawn list to place`);
    await fetchCustomCatalog();
    console.log("[import] customCatalog now has", state.customCatalog?.length, "entries");
    // Force spawn section open and make sure the right tool/panel is visible.
    if (state.toolMode !== "select") {
      setToolMode("select");
    }
    ui.spawnSection.open = true;
    renderSpawnList();
  } catch (err) {
    console.error("[import] FAILED:", err);
    setStatus(`Import failed: ${err.message}`);
  }
}

function escapeHtml(s) {
  return s.replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"})[c]);
}

function spawnObjectAtHover(entry) {
  if (!state.hover) {
    setStatus("Aim cursor at terrain first");
    return;
  }
  const hover = state.hover;
  // Place the object in the region actually under the cursor, not the
  // initial-anchor region. Falls back to base when hover.region is missing.
  const hoverRegion = hover.region;
  const regionX = hoverRegion ? hoverRegion.x : state.baseX;
  const regionY = hoverRegion ? hoverRegion.y : state.baseY;
  const offX = regionOffsetX(regionX);
  const offZ = regionOffsetZ(regionY);
  const localX = offX + 960 - hover.x;
  const localY = hover.y;
  const localZ = hover.z - offZ + 960;
  const regionID = ((regionY & 0xff) << 8) | (regionX & 0xff);
  const isCustom = !!entry.isCustom || entry.id >= 0x80000000;
  const placement = {
    _cid: state.nextClientID++,
    objID: entry.id,
    uid: -1,
    regionID,
    regionX,
    regionY,
    localX,
    localY,
    localZ,
    originalX: localX,
    originalY: localY,
    originalZ: localZ,
    originalYaw: 0,
    wx: hover.x,
    wy: hover.y,
    wz: hover.z,
    yaw: 0,
    big: false,
    isCpd: !!entry.isCpd,
    isCustom,
    objectPath: entry.path,
    isNew: true
  };
  state.objectPlacements.push(placement);
  const newIdx = state.objectPlacements.length - 1;
  state.selection = { index: newIdx };
  rebuildObjectMarkers();
  updateSelectionUI();
  refreshSaveDirtyState();
  if (!state.objectAssets.has(entry.id)) {
    state.objectAssets.set(entry.id, { state: "loading", meshes: null });
    fetchObjectAsset(entry.id);
  }
  const snapshot = JSON.parse(JSON.stringify(placement));
  pushUndo({
    label: `Spawn ${entry.path}`,
    undo: () => {
      const i = findPlacementByCid(snapshot._cid);
      if (i < 0) return;
      removePlacementAt(i);
      rebuildObjectMarkers();
      refreshSaveDirtyState();
    },
    redo: () => {
      state.objectPlacements.push({ ...snapshot });
      rebuildObjectMarkers();
      refreshSaveDirtyState();
    }
  });
  setStatus(`Spawned ${entry.path} at hover`);
}

async function restoreLoadedRegionNVMs() {
  const regions = [...state.regions.values()];
  if (regions.length === 0) {
    setStatus("Load a region first");
    return;
  }
  ui.restoreNVMBtn.disabled = true;
  ui.bakeStatus.textContent = `Restoring NVM for ${regions.length} region(s)...`;
  try {
    let restored = 0, skipped = 0;
    for (const region of regions) {
      const result = await fetchJSON(`/api/region/restore-nvm?x=${region.x}&y=${region.y}`, { method: "POST" });
      if (result.restored > 0) {
        restored++;
        loadRegionNVMCells(region);
      } else {
        skipped++;
      }
    }
    ui.bakeStatus.textContent = `Restored ${restored} NVM · skipped ${skipped} (no .bak)`;
    setStatus(`Restored ${restored} NVM(s)`);
  } catch (err) {
    ui.bakeStatus.textContent = `Restore failed: ${err.message}`;
    setStatus(`Restore failed: ${err.message}`);
  } finally {
    ui.restoreNVMBtn.disabled = false;
  }
}

async function rebuildLoadedRegionNVMs() {
  const regions = [...state.regions.values()];
  if (regions.length === 0) {
    setStatus("Load a region first");
    return;
  }
  if (state.dirty.size > 0) {
    ui.bakeStatus.textContent = "Saving terrain edits...";
    await saveDirtyRegions({ silent: true });
  }
  if (hasUnsavedCustomPlacements()) {
    ui.bakeStatus.textContent = "Saving custom object edits...";
    await saveCustomPlacements({ silent: true });
  }
  if (typeof hasUnsavedObjectChanges === "function" && hasUnsavedObjectChanges()) {
    ui.bakeStatus.textContent = "Saving object edits...";
    await saveObjectEdits({ silent: true });
  }
  await rebuildMapOnlyNVMs(regions, {
    createMissing: !!ui.nvmCreateMissing?.checked,
    statusPrefix: "Manual NVM"
  });
}

async function rebuildMapOnlyNVMs(regions, opts = {}) {
  const parsedSlope = parseFloat(ui.nvmSlope.value);
  const slope = Number.isFinite(parsedSlope) ? parsedSlope : 60;
  if (ui.rebuildNVMBtn) ui.rebuildNVMBtn.disabled = true;
  ui.bakeStatus.textContent = `Rebuilding NVM for ${regions.length} region(s)...`;
  const startedAt = performance.now();
  try {
    let done = 0, skipped = 0, neighborEdges = 0;
    for (const region of regions) {
      const params = new URLSearchParams({
        x: region.x, y: region.y, slope: slope
      });
      params.set("full", "1");
      if (opts.createMissing) params.set("create", "1");
      try {
        const result = await fetchJSON(`/api/region/rebuild-nvm?${params}`, { method: "POST" });
        done++;
        neighborEdges += result.neighborEdges || 0;
        const tags = [];
        if (result.createdNew) tags.push("created");
        if (result.mode) tags.push(result.mode);
        if (result.neighborEdges) tags.push(`edge-sync ${result.neighborEdges}`);
        const tag = tags.length ? `(${tags.join(", ")})` : "";
        ui.bakeStatus.textContent = `NVM ${done}/${regions.length} ${tag} · cells=${result.cells} open=${result.openCells} edges=${result.internalEdges} (walkable ${result.walkableTiles}/${result.totalTiles})`;
        const loaded = state.regions.get(keyFor(region.x, region.y));
        if (loaded) loadRegionNVMCells(loaded);
      } catch (err) {
        if (String(err.message).includes("no NVM") || String(err.message).includes("mesh not found")) {
          skipped++;
        } else {
          throw err;
        }
      }
    }
    const elapsed = ((performance.now() - startedAt) / 1000).toFixed(1);
    const syncText = neighborEdges ? ` - synced ${neighborEdges} neighbor edge file(s)` : "";
    ui.bakeStatus.textContent = `Rebuilt ${done} NVM(s)${syncText} - skipped ${skipped} (no NVM file) in ${elapsed}s`;
    setStatus(`${opts.statusPrefix || "NVM"} rebuilt for ${done} region(s)`);
    return { done, skipped };
  } catch (err) {
    ui.bakeStatus.textContent = `Rebuild failed: ${err.message}`;
    setStatus(`Rebuild failed: ${err.message}`);
    throw err;
  } finally {
    if (ui.rebuildNVMBtn) ui.rebuildNVMBtn.disabled = false;
  }
}

async function restoreLoadedRegionLightmaps() {
  const regions = [...state.regions.values()];
  if (regions.length === 0) {
    setStatus("Load a region first");
    return;
  }
  ui.restoreLightmapBtn.disabled = true;
  ui.bakeStatus.textContent = `Restoring ${regions.length} region(s)...`;
  try {
    let restored = 0;
    let skipped = 0;
    for (const region of regions) {
      const result = await fetchJSON(`/api/region/restore-lightmap?x=${region.x}&y=${region.y}`, {
        method: "POST"
      });
      if (result.restored) {
        restored++;
        loadRegionLightmap(region, true);
      } else {
        skipped++;
      }
    }
    ui.bakeStatus.textContent = `Restored ${restored} · skipped ${skipped} (no backup)`;
    setStatus(`Restored ${restored} lightmap(s)`);
  } catch (err) {
    ui.bakeStatus.textContent = `Restore failed: ${err.message}`;
    setStatus(`Restore failed: ${err.message}`);
  } finally {
    ui.restoreLightmapBtn.disabled = false;
  }
}

async function bakeLoadedRegionShadows() {
  const regions = [...state.regions.values()];
  if (regions.length === 0) {
    setStatus("Load a region first");
    return;
  }
  // Bake reads .m and .o2 from disk, so flush any pending edits first.
  if (state.dirty.size > 0) {
    ui.bakeStatus.textContent = "Saving terrain edits...";
    await saveDirtyRegions();
  }
  if (hasUnsavedCustomPlacements()) {
    ui.bakeStatus.textContent = "Saving custom object edits...";
    await saveCustomPlacements();
  }
  if (typeof hasUnsavedObjectChanges === "function" && hasUnsavedObjectChanges()) {
    ui.bakeStatus.textContent = "Saving object edits...";
    await saveObjectEdits();
  }
  const az = parseFloat(ui.bakeAzimuth.value);
  const el = parseFloat(ui.bakeElevation.value);
  const samples = parseInt(ui.bakeSamples.value, 10) || 1;
  const softness = parseFloat(ui.bakeSoftness.value) || 0;
  ui.bakeShadowsBtn.disabled = true;
  ui.bakeStatus.textContent = `Baking ${regions.length} region(s)...`;
  const startedAt = performance.now();
  try {
    let done = 0;
    for (const region of regions) {
      const params = new URLSearchParams({
        x: region.x, y: region.y,
        azimuth: az, elevation: el,
        samples: samples, softness: softness
      });
      const result = await fetchJSON(`/api/region/bake-shadows?${params}`, { method: "POST" });
      done++;
      ui.bakeStatus.textContent = `Baked ${done}/${regions.length} · last: ${result.placements} placements · ${result.triangles} tris`;
      loadRegionLightmap(region, true);
    }
    const elapsed = ((performance.now() - startedAt) / 1000).toFixed(1);
    ui.bakeStatus.textContent = `Baked ${done} region(s) in ${elapsed}s`;
    setStatus(`Shadows rebaked`);
  } catch (err) {
    ui.bakeStatus.textContent = `Bake failed: ${err.message}`;
    setStatus(`Bake failed: ${err.message}`);
  } finally {
    ui.bakeShadowsBtn.disabled = false;
  }
}

async function saveObjectEdits(opts = {}) {
  if (!hasUnsavedObjectChanges()) {
    if (!opts.silent) setStatus("Nothing to save");
    return [];
  }
  // Group all changes by the region whose .o2 stores them.
  const groups = new Map();
  function ensure(regionID) {
    if (!groups.has(regionID)) groups.set(regionID, { edits: [], deletes: [], adds: [], pendingAddRefs: [] });
    return groups.get(regionID);
  }
  // Custom placements are persisted via the sidecar saveCustomPlacements
  // flow; filter them out of the .o2 save path here.
  // For placements that dragged across a region boundary since load
  // (originalRegionID != current regionID), turn the edit into a
  // delete-from-old-region + add-to-new-region pair — the server can't
  // edit-by-key across regions because each region's .o2 is independent.
  const crossRegionMoves = []; // tracks adds we need to re-tag with new UIDs
  for (const idx of state.dirtyPlacements) {
    const p = state.objectPlacements[idx];
    if (p.isCustom || p.isNew) continue;
    if (p.originalRegionID !== undefined && p.originalRegionID !== p.regionID) {
      ensure(p.originalRegionID).deletes.push({
        objID: p.objID, uid: p.originalUID ?? p.uid, regionID: p.originalRegionID
      });
      const g = ensure(p.regionID);
      g.adds.push({
        objID: p.objID, regionID: p.regionID,
        x: p.localX, y: p.localY, z: p.localZ, yaw: p.yaw,
        big: !!p.big, struct: false
      });
      g.pendingAddRefs.push(idx);
      crossRegionMoves.push(idx);
      continue;
    }
    ensure(p.regionID).edits.push({
      objID: p.objID, uid: p.uid, regionID: p.regionID,
      x: p.localX, y: p.localY, z: p.localZ, yaw: p.yaw
    });
  }
  for (const d of state.pendingDeletes) {
    if (d.objID >= 0x80000000) continue;
    ensure(d.regionID).deletes.push({ objID: d.objID, uid: d.uid, regionID: d.regionID });
  }
  for (let i = 0; i < state.objectPlacements.length; i++) {
    const p = state.objectPlacements[i];
    if (!p.isNew || p.isCustom) continue;
    const g = ensure(p.regionID);
    g.adds.push({
      objID: p.objID, regionID: p.regionID,
      x: p.localX, y: p.localY, z: p.localZ, yaw: p.yaw,
      big: !!p.big, struct: false
    });
    g.pendingAddRefs.push(i);
  }
  if (groups.size === 0) {
    if (!opts.silent) setStatus("Nothing to save");
    return [];
  }

  try {
    let totalUpdated = 0, totalDeleted = 0, totalAdded = 0;
    const affected = [];
    for (const [regionID, change] of groups) {
      const rx = regionID & 0xff;
      const ry = (regionID >> 8) & 0xff;
      affected.push({ x: rx, y: ry });
      const result = await fetchJSON("/api/region/objects/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ x: rx, y: ry, edits: change.edits, deletes: change.deletes, adds: change.adds })
      });
      totalUpdated += result.updated || 0;
      totalDeleted += result.deleted || 0;
      const added = result.added || [];
      totalAdded += added.length;
      // Apply server-assigned UIDs to the new placements.
      for (let i = 0; i < change.pendingAddRefs.length && i < added.length; i++) {
        const refIdx = change.pendingAddRefs[i];
        const p = state.objectPlacements[refIdx];
        p.uid = added[i].uid;
        p.isNew = false;
        p.originalX = p.localX;
        p.originalY = p.localY;
        p.originalZ = p.localZ;
        p.originalYaw = p.yaw;
        // Snapshot the new region home so future drags-across work too.
        p.originalRegionID = p.regionID;
        p.originalUID = p.uid;
      }
    }
    const nextDirty = new Set();
    for (const idx of state.dirtyPlacements) {
      const p = state.objectPlacements[idx];
      if (p?.isCustom) nextDirty.add(idx);
    }
    state.dirtyPlacements = nextDirty;
    state.pendingDeletes = state.pendingDeletes.filter(d => d.objID >= 0x80000000);
    for (const p of state.objectPlacements) {
      if (p.isCustom) continue;
      p.originalX = p.localX;
      p.originalY = p.localY;
      p.originalZ = p.localZ;
      p.originalYaw = p.yaw;
      p.originalRegionID = p.regionID;
      p.originalUID = p.uid;
    }
    refreshSaveDirtyState();
    if (!opts.silent) {
      setStatus(`Saved · updated ${totalUpdated} · deleted ${totalDeleted} · added ${totalAdded}`);
    }
    return affected;
  } catch (err) {
    if (!opts.silent) setStatus(`Save failed: ${err.message}`);
    throw err;
  }
}

function drawSelectionBox(viewProj) {
  if (!state.selection) return;
  const p = state.objectPlacements[state.selection.index];
  const aabb = placementWorldAABB(p);
  const x0 = aabb[0], y0 = aabb[1], z0 = aabb[2];
  const x1 = aabb[3], y1 = aabb[4], z1 = aabb[5];
  const corners = [
    x0, y0, z0,  x1, y0, z0,  x1, y1, z0,  x0, y1, z0,
    x0, y0, z1,  x1, y0, z1,  x1, y1, z1,  x0, y1, z1
  ];
  const edges = [
    0,1, 1,2, 2,3, 3,0,
    4,5, 5,6, 6,7, 7,4,
    0,4, 1,5, 2,6, 3,7
  ];
  const verts = new Float32Array(edges.length * 3);
  for (let i = 0; i < edges.length; i++) {
    const c = edges[i] * 3;
    verts[i * 3] = corners[c];
    verts[i * 3 + 1] = corners[c + 1];
    verts[i * 3 + 2] = corners[c + 2];
  }
  gl.bindBuffer(gl.ARRAY_BUFFER, selectionBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, verts, gl.DYNAMIC_DRAW);
  gl.useProgram(lineProgram.program);
  gl.uniformMatrix4fv(lineProgram.uniforms.uViewProj, false, viewProj);
  gl.uniform3fv(lineProgram.uniforms.uColor, [1.0, 0.85, 0.15]);
  bindAttribute(lineProgram.attributes.aPosition, selectionBuffer, 3);
  gl.disable(gl.DEPTH_TEST);
  gl.drawArrays(gl.LINES, 0, edges.length);
  gl.enable(gl.DEPTH_TEST);
}

function frame(now) {
  const dt = Math.min(0.05, (now - state.lastFrame) / 1000);
  state.lastFrame = now;
  resize();
  updateCamera(dt);
  updateHover();
  updateSelectedObject(dt);
  streamLoadTick();
  if (state.toolMode === "brush" && state.leftDown) stampBrush(false);
  draw();
  drawWorldMap();
  requestAnimationFrame(frame);
}

function updateCamera(dt) {
  const basis = cameraBasis();
  const fast = state.keys.has("KeyR");
  const speed = fast ? 1200 : 700;
  const step = speed * dt;
  if (state.keys.has("KeyW")) moveVec(basis.forward, step);
  if (state.keys.has("KeyS")) moveVec(basis.forward, -step);
  if (state.keys.has("KeyD")) moveVec(basis.right, step);
  if (state.keys.has("KeyA")) moveVec(basis.right, -step);
  if (state.keys.has("Space")) state.camera.y += step;
  if (state.keys.has("ShiftLeft") || state.keys.has("ShiftRight")) state.camera.y -= step;
}

function moveVec(v, amount) {
  state.camera.x += v[0] * amount;
  state.camera.y += v[1] * amount;
  state.camera.z += v[2] * amount;
}

function updateHover() {
  const ray = mouseRay();
  let best = null;
  for (let t = 20; t < 9000; t += 18) {
    const x = state.camera.x + ray[0] * t;
    const y = state.camera.y + ray[1] * t;
    const z = state.camera.z + ray[2] * t;
    const h = heightAtWorld(x, z);
    if (h && y <= h.height + 3) {
      best = { x, y: h.height, z, region: h.region };
      break;
    }
  }
  state.hover = best;
  updateCursorCoordsReadout();
}

function updateCursorCoordsReadout() {
  if (!ui.cursorCoords) return;
  if (!state.hover) {
    ui.cursorCoords.textContent = "-";
    return;
  }
  const region = state.hover.region;
  if (!region) {
    ui.cursorCoords.textContent = "-";
    return;
  }
  const offX = regionOffsetX(region.x);
  const offZ = regionOffsetZ(region.y);
  const localX = offX + 960 - state.hover.x;
  const localZ = state.hover.z - offZ + 960;
  const gx = Math.max(0, Math.min(GRID_SIZE - 1, Math.round(localX / CELL_SIZE)));
  const gz = Math.max(0, Math.min(GRID_SIZE - 1, Math.round(localZ / CELL_SIZE)));
  ui.cursorCoords.textContent =
    `region ${region.x},${region.y}  local (${localX.toFixed(0)}, ${state.hover.y.toFixed(0)}, ${localZ.toFixed(0)})  grid ${gx},${gz}`;
}

function stampBrush(force) {
  if (!state.hover) return;
  const now = performance.now();
  if (!force && now - state.lastBrushAt < 75) return;
  state.lastBrushAt = now;
  const radius = parseFloat(ui.brushRadius.value);

  if (state.brushMode === "tile") {
    stampTileBrush(radius);
    return;
  }

  const strength = parseFloat(ui.brushStrength.value);
  const isEqualize = state.brushMode === "equalize";
  if (isEqualize && state.equalizeTarget == null) {
    setStatus("Middle-click terrain first to sample a target height");
    return;
  }
  const delta = (state.brushMode === "raise" ? 1 : -1) * strength;
  // Equalize blends toward target with a per-stamp rate proportional to
  // strength (kept under 1 so flattening is smooth, not snappy).
  const equalizeRate = Math.min(1, strength / 20);
  let changedAny = false;

  for (const region of state.regions.values()) {
    const offX = regionOffsetX(region.x);
    const offZ = regionOffsetZ(region.y);
    const regionKey = keyFor(region.x, region.y);
    let changed = false;
    for (let gz = 0; gz < GRID_SIZE; gz++) {
      for (let gx = 0; gx < GRID_SIZE; gx++) {
        const wx = offX + 960 - gx * CELL_SIZE;
        const wz = offZ - 960 + gz * CELL_SIZE;
        const dx = wx - state.hover.x;
        const dz = wz - state.hover.z;
        const dist = Math.hypot(dx, dz);
        if (dist > radius) continue;
        const t = dist / radius;
        const weight = 1 - t * t * (3 - 2 * t);
        const idx = gz * GRID_SIZE + gx;
        const before = region.heights[idx];
        let after;
        if (isEqualize) {
          const k = equalizeRate * weight;
          after = before + (state.equalizeTarget - before) * k;
        } else {
          after = before + delta * weight;
        }
        if (after === before) continue;
        recordBrushVertex(regionKey, idx, before);
        region.heights[idx] = after;
        changed = true;
      }
    }
    if (changed) {
      buildTerrainMesh(region);
      state.dirty.add(regionKey);
      changedAny = true;
    }
  }
  if (changedAny) updateMeta();
}

function stampTileBrush(radius) {
  if (state.activeTileID === null) {
    setStatus("Pick a tile in the panel first");
    return;
  }
  const id = state.activeTileID & 0x3FF;
  let changedAny = false;
  for (const region of state.regions.values()) {
    const offX = regionOffsetX(region.x);
    const offZ = regionOffsetZ(region.y);
    const regionKey = keyFor(region.x, region.y);
    let changed = false;
    for (let gz = 0; gz < GRID_SIZE; gz++) {
      for (let gx = 0; gx < GRID_SIZE; gx++) {
        const wx = offX + 960 - gx * CELL_SIZE;
        const wz = offZ - 960 + gz * CELL_SIZE;
        const dx = wx - state.hover.x;
        const dz = wz - state.hover.z;
        if (Math.hypot(dx, dz) > radius) continue;
        const idx = gz * GRID_SIZE + gx;
        if (region.tileIDs[idx] !== id) {
          recordBrushVertex(regionKey, idx, region.tileIDs[idx]);
          region.tileIDs[idx] = id;
          changed = true;
        }
      }
    }
    if (changed) {
      state.dirty.add(regionKey);
      state.tileDirty.add(regionKey);
      changedAny = true;
    }
  }
  if (changedAny) updateMeta();
}

async function saveDirtyRegions(opts = {}) {
  const keys = [...state.dirty];
  if (keys.length === 0) {
    if (!opts.silent) setStatus("No dirty regions to save");
    return [];
  }
  try {
    let saved = 0;
    const affected = [];
    for (const key of keys) {
      const region = state.regions.get(key);
      if (!region) continue;
      affected.push({ x: region.x, y: region.y });
      const includeTiles = state.tileDirty.has(key);
      const body = {
        x: region.x,
        y: region.y,
        heights: Array.from(region.heights),
        syncNvm: true
      };
      if (includeTiles) body.textureIDs = Array.from(region.tileIDs);
      await fetchJSON("/api/region/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
      });
      state.dirty.delete(key);
      if (includeTiles) {
        state.tileDirty.delete(key);
        region.textureURL = `/api/region/texture?x=${region.x}&y=${region.y}&v=${Date.now()}`;
        loadRegionTexture(region);
      }
      saved++;
      if (!opts.silent) setStatus(`Saved ${saved}/${keys.length}`);
      updateMeta();
    }
    if (!opts.silent) setStatus(`Saved ${saved} region(s)`);
    return affected;
  } catch (err) {
    if (!opts.silent) setStatus(`Save failed: ${err.message}`);
    throw err;
  }
}

function draw() {
  const viewProj = viewProjectionMatrix();
  gl.viewport(0, 0, canvas.width, canvas.height);
  gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);

  gl.useProgram(terrainProgram.program);
  gl.uniformMatrix4fv(terrainProgram.uniforms.uViewProj, false, viewProj);
  gl.uniform1i(terrainProgram.uniforms.uTexture, 0);
  gl.uniform1i(terrainProgram.uniforms.uLightmap, 1);
  gl.disable(gl.CULL_FACE);
  gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, terrainIndexBuffer);
  for (const region of state.regions.values()) {
    bindAttribute(terrainProgram.attributes.aPosition, region.mesh.position, 3);
    bindAttribute(terrainProgram.attributes.aColor, region.mesh.color, 3);
    bindAttribute(terrainProgram.attributes.aNormal, region.mesh.normal, 3);
    bindAttribute(terrainProgram.attributes.aTexCoord, region.mesh.texCoord, 2);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, region.texture || whitePixelTexture);
    gl.uniform1f(terrainProgram.uniforms.uHasTexture, region.texture ? 1.0 : 0.0);
    gl.activeTexture(gl.TEXTURE1);
    gl.bindTexture(gl.TEXTURE_2D, region.lightmap || whitePixelTexture);
    const lmStrength = region.lightmap ? state.lightmapStrength : 0.0;
    gl.uniform1f(terrainProgram.uniforms.uLightmapStrength, lmStrength);
    gl.drawElements(gl.TRIANGLES, terrainIndexCount, gl.UNSIGNED_SHORT, 0);
  }

  drawObjectMeshes(viewProj);
  if (state.showObjectCollision) drawObjectCollisionOverlay(viewProj);

  if (state.objects.count > 0) {
    gl.useProgram(pointProgram.program);
    gl.uniformMatrix4fv(pointProgram.uniforms.uViewProj, false, viewProj);
    bindAttribute(pointProgram.attributes.aPosition, objectPositionBuffer, 3);
    bindAttribute(pointProgram.attributes.aColor, objectColorBuffer, 3);
    gl.drawArrays(gl.POINTS, 0, state.objects.count);
  }

  drawSelectionBox(viewProj);

  if (state.showNVMOverlay) drawNVMOverlay(viewProj);
  if (state.showHandoffOverlay) drawSelectedHandoffOverlay(viewProj);

  if (state.toolMode === "brush" && state.hover) drawBrushCircle(viewProj);
}

function drawNVMOverlay(viewProj) {
  gl.useProgram(lineProgram.program);
  gl.uniformMatrix4fv(lineProgram.uniforms.uViewProj, false, viewProj);
  gl.disable(gl.DEPTH_TEST);
  // Open cells in green
  gl.uniform3fv(lineProgram.uniforms.uColor, [0.35, 0.95, 0.55]);
  for (const region of state.regions.values()) {
    if (region.nvmOpenBuffer && region.nvmOpenVerts > 0) {
      bindAttribute(lineProgram.attributes.aPosition, region.nvmOpenBuffer, 3);
      gl.drawArrays(gl.LINES, 0, region.nvmOpenVerts);
    }
  }
  // Closed cells in red
  gl.uniform3fv(lineProgram.uniforms.uColor, [0.95, 0.4, 0.4]);
  for (const region of state.regions.values()) {
    if (region.nvmClosedBuffer && region.nvmClosedVerts > 0) {
      bindAttribute(lineProgram.attributes.aPosition, region.nvmClosedBuffer, 3);
      gl.drawArrays(gl.LINES, 0, region.nvmClosedVerts);
    }
  }
  gl.enable(gl.DEPTH_TEST);
}

function drawSelectedHandoffOverlay(viewProj) {
  if (!state.selection) return;
  const placement = state.objectPlacements[state.selection.index];
  if (!placement) return;

  const openFill = [];
  const closedFill = [];
  const openLines = [];
  const closedLines = [];

  for (const region of state.regions.values()) {
    const objectIndex = matchingNVMObjectIndex(region, placement);
    if (objectIndex < 0) continue;
    const cells = region.nvmCells || [];
    for (const cell of cells) {
      const indices = cell.objectIndices || [];
      if (!indices.includes(objectIndex)) continue;
      const fill = cell.open ? openFill : closedFill;
      const lines = cell.open ? openLines : closedLines;
      pushNVMCellSurface(fill, lines, region, cell, 8);
    }
  }

  if (openFill.length === 0 && closedFill.length === 0) return;

  gl.disable(gl.DEPTH_TEST);
  gl.enable(gl.BLEND);
  gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

  gl.useProgram(flatProgram.program);
  gl.uniformMatrix4fv(flatProgram.uniforms.uViewProj, false, viewProj);
  drawFlatBatch(handoffOpenFillBuffer, openFill, [0.0, 0.72, 1.0, 0.28]);
  drawFlatBatch(handoffClosedFillBuffer, closedFill, [1.0, 0.52, 0.08, 0.24]);

  gl.disable(gl.BLEND);
  gl.useProgram(lineProgram.program);
  gl.uniformMatrix4fv(lineProgram.uniforms.uViewProj, false, viewProj);
  drawLineBatch(handoffOpenLineBuffer, openLines, [0.0, 0.9, 1.0]);
  drawLineBatch(handoffClosedLineBuffer, closedLines, [1.0, 0.62, 0.1]);
  gl.enable(gl.DEPTH_TEST);
}

function matchingNVMObjectIndex(region, placement) {
  const objects = region.nvmObjects || [];
  for (const obj of objects) {
    if (Number(obj.assetID) !== Number(placement.objID)) continue;
    if (Number(obj.uid) !== Number(placement.uid)) continue;
    if (Number(obj.regionID) !== Number(placement.regionID)) continue;
    return Number(obj.index);
  }
  return -1;
}

function pushNVMCellSurface(fill, lines, region, cell, yOffset) {
  const corners = nvmCellWorldCorners(region, cell, yOffset);
  pushTri(fill, corners[0], corners[1], corners[2]);
  pushTri(fill, corners[0], corners[2], corners[3]);
  pushLine(lines, corners[0], corners[1]);
  pushLine(lines, corners[1], corners[2]);
  pushLine(lines, corners[2], corners[3]);
  pushLine(lines, corners[3], corners[0]);
}

function nvmCellWorldCorners(region, cell, yOffset) {
  const offX = regionOffsetX(region.x);
  const offZ = regionOffsetZ(region.y);
  const w0X = offX + 960 - cell.maxX;
  const w1X = offX + 960 - cell.minX;
  const w0Z = offZ - 960 + cell.minZ;
  const w1Z = offZ - 960 + cell.maxZ;
  return [[w0X, w0Z], [w1X, w0Z], [w1X, w1Z], [w0X, w1Z]].map(([x, z]) => {
    const h = heightAtWorld(x, z);
    return [x, (h ? h.height : 0) + yOffset, z];
  });
}

function drawFlatBatch(buffer, verts, color) {
  if (verts.length === 0) return;
  gl.uniform4fv(flatProgram.uniforms.uColor, color);
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(verts), gl.DYNAMIC_DRAW);
  bindAttribute(flatProgram.attributes.aPosition, buffer, 3);
  gl.drawArrays(gl.TRIANGLES, 0, verts.length / 3);
}

function drawLineBatch(buffer, verts, color) {
  if (verts.length === 0) return;
  gl.uniform3fv(lineProgram.uniforms.uColor, color);
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(verts), gl.DYNAMIC_DRAW);
  bindAttribute(lineProgram.attributes.aPosition, buffer, 3);
  gl.drawArrays(gl.LINES, 0, verts.length / 3);
}

function drawObjectCollisionOverlay(viewProj) {
  if (state.objectPlacements.length === 0) return;
  const fill = [];
  const lines = [];
  for (const placement of state.objectPlacements) {
    const asset = state.objectAssets.get(placement.objID);
    if (!asset || asset.state !== "ready") continue;
    if (!asset.hasCollision) continue;
    if (asset.collisionNavVertices && asset.collisionNavIndices && asset.collisionNavIndices.length >= 3) {
      pushCollisionNavMesh(
        fill,
        lines,
        placement,
        asset.collisionNavVertices,
        asset.collisionNavIndices,
        asset.collisionNavOutlineIndices
      );
      continue;
    }
    const min = asset.collisionBBoxMin;
    const max = asset.collisionBBoxMax;
    if (!min || !max) continue;
    const corners = collisionQuadCorners(placement, min, max);
    if (!corners) continue;
    pushTri(fill, corners[0], corners[1], corners[2]);
    pushTri(fill, corners[0], corners[2], corners[3]);
    pushLine(lines, corners[0], corners[1]);
    pushLine(lines, corners[1], corners[2]);
    pushLine(lines, corners[2], corners[3]);
    pushLine(lines, corners[3], corners[0]);
  }
  if (fill.length === 0) return;

  gl.disable(gl.DEPTH_TEST);
  gl.enable(gl.BLEND);
  gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

  gl.useProgram(flatProgram.program);
  gl.uniformMatrix4fv(flatProgram.uniforms.uViewProj, false, viewProj);
  gl.uniform4fv(flatProgram.uniforms.uColor, [1.0, 0.05, 0.02, 0.32]);
  gl.bindBuffer(gl.ARRAY_BUFFER, collisionFillBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(fill), gl.DYNAMIC_DRAW);
  bindAttribute(flatProgram.attributes.aPosition, collisionFillBuffer, 3);
  gl.drawArrays(gl.TRIANGLES, 0, fill.length / 3);

  gl.disable(gl.BLEND);
  gl.useProgram(lineProgram.program);
  gl.uniformMatrix4fv(lineProgram.uniforms.uViewProj, false, viewProj);
  gl.uniform3fv(lineProgram.uniforms.uColor, [1.0, 0.18, 0.12]);
  gl.bindBuffer(gl.ARRAY_BUFFER, collisionLineBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array(lines), gl.DYNAMIC_DRAW);
  bindAttribute(lineProgram.attributes.aPosition, collisionLineBuffer, 3);
  gl.drawArrays(gl.LINES, 0, lines.length / 3);
  gl.enable(gl.DEPTH_TEST);
}

function pushCollisionNavMesh(fill, lines, placement, vertices, indices, outlineIndices) {
  const world = [];
  for (let i = 0; i + 2 < vertices.length; i += 3) {
    world.push(collisionLocalPoint(placement, vertices[i], vertices[i + 1], vertices[i + 2]));
  }
  for (let i = 0; i + 2 < indices.length; i += 3) {
    const a = world[indices[i]];
    const b = world[indices[i + 1]];
    const c = world[indices[i + 2]];
    if (!a || !b || !c) continue;
    pushTri(fill, a, b, c);
  }
  if (outlineIndices && outlineIndices.length >= 2) {
    for (let i = 0; i + 1 < outlineIndices.length; i += 2) {
      const a = world[outlineIndices[i]];
      const b = world[outlineIndices[i + 1]];
      if (a && b) pushLine(lines, a, b);
    }
    return;
  }
  const seen = new Set();
  for (let i = 0; i + 2 < indices.length; i += 3) {
    pushIndexedCollisionLine(lines, seen, world, indices[i], indices[i + 1]);
    pushIndexedCollisionLine(lines, seen, world, indices[i + 1], indices[i + 2]);
    pushIndexedCollisionLine(lines, seen, world, indices[i + 2], indices[i]);
  }
}

function pushIndexedCollisionLine(lines, seen, world, aIdx, bIdx) {
  const lo = Math.min(aIdx, bIdx);
  const hi = Math.max(aIdx, bIdx);
  const key = `${lo}:${hi}`;
  if (seen.has(key)) return;
  seen.add(key);
  const a = world[aIdx];
  const b = world[bIdx];
  if (a && b) pushLine(lines, a, b);
}

function collisionQuadCorners(placement, min, max) {
  const local = [
    [min[0], min[1], min[2]],
    [max[0], min[1], min[2]],
    [max[0], min[1], max[2]],
    [min[0], min[1], max[2]]
  ];
  return local.map(([x, y, z]) => collisionLocalPoint(placement, x, y, z));
}

function collisionLocalPoint(placement, x, y, z) {
  const c = Math.cos(placement.yaw || 0);
  const s = Math.sin(placement.yaw || 0);
  return [
    -c * x + s * z + placement.wx,
    placement.wy + (y || 0) + 3,
    s * x + c * z + placement.wz
  ];
}

function pushTri(out, a, b, c) {
  out.push(a[0], a[1], a[2], b[0], b[1], b[2], c[0], c[1], c[2]);
}

function pushLine(out, a, b) {
  out.push(a[0], a[1], a[2], b[0], b[1], b[2]);
}

function drawObjectMeshes(viewProj) {
  if (state.objectPlacements.length === 0) return;
  gl.useProgram(objectProgram.program);
  gl.uniform1i(objectProgram.uniforms.uTexture, 0);
  gl.activeTexture(gl.TEXTURE0);
  const mvp = new Float32Array(16);
  const tmp = new Float32Array(16);
  for (const placement of state.objectPlacements) {
    const asset = state.objectAssets.get(placement.objID);
    if (!asset || asset.state !== "ready" || !asset.meshes) continue;
    const model = makeObjectMatrix(placement.wx, placement.wy, placement.wz, placement.yaw, tmp);
    mat4Multiply(viewProj, model, mvp);
    gl.uniformMatrix4fv(objectProgram.uniforms.uMVP, false, mvp);
    gl.uniform3fv(objectProgram.uniforms.uTint, [0.7, 0.65, 0.55]);
    for (const mesh of asset.meshes) {
      bindAttributeStride(objectProgram.attributes.aPosition, mesh.vbuf, 3, 5 * 4, 0);
      if (objectProgram.attributes.aTexCoord !== undefined) {
        bindAttributeStride(objectProgram.attributes.aTexCoord, mesh.vbuf, 2, 5 * 4, 3 * 4);
      }
      gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, mesh.ibuf);
      if (mesh.texture) {
        gl.bindTexture(gl.TEXTURE_2D, mesh.texture);
        gl.uniform1f(objectProgram.uniforms.uHasTexture, 1.0);
      } else {
        gl.bindTexture(gl.TEXTURE_2D, whitePixelTexture);
        gl.uniform1f(objectProgram.uniforms.uHasTexture, 0.0);
      }
      gl.drawElements(gl.TRIANGLES, mesh.count, gl.UNSIGNED_SHORT, 0);
    }
  }
}

// ---------- World Map (M key) ----------

const WORLD_MAP_W = 256;     // region grid: 256 X, 128 Y
const WORLD_MAP_H = 128;
const WORLD_MAP_PX = 4;      // pixels per region in the backing canvas

function toggleWorldMap(force) {
  const next = typeof force === "boolean" ? force : !state.worldMapOpen;
  state.worldMapOpen = next;
  ui.worldMap.hidden = !next;
  if (next) renderWorldMap();
}

function renderWorldMap() {
  const cv = ui.worldMapCanvas;
  // Use a fixed internal-pixel viewport; zoom/pan are applied as a transform.
  cv.width = WORLD_MAP_W * WORLD_MAP_PX;
  cv.height = WORLD_MAP_H * WORLD_MAP_PX;
  const ctx = cv.getContext("2d");
  ctx.imageSmoothingEnabled = false;
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.fillStyle = "#0a0c0a";
  ctx.fillRect(0, 0, cv.width, cv.height);

  ctx.setTransform(state.worldMapZoom, 0, 0, state.worldMapZoom,
    state.worldMapPanX, state.worldMapPanY);

  if (state.info?.regions) {
    for (const r of state.info.regions) {
      if (r.isDungeon) continue;
      const color = r.hasRef ? "#c9a149" : "#3f6e3f";
      ctx.fillStyle = color;
      ctx.fillRect(r.x * WORLD_MAP_PX, (WORLD_MAP_H - 1 - r.y) * WORLD_MAP_PX, WORLD_MAP_PX, WORLD_MAP_PX);
    }
  }

  ctx.strokeStyle = "#d5a241";
  ctx.lineWidth = 1 / state.worldMapZoom;
  for (const region of state.regions.values()) {
    ctx.strokeRect(
      region.x * WORLD_MAP_PX + 0.5,
      (WORLD_MAP_H - 1 - region.y) * WORLD_MAP_PX + 0.5,
      WORLD_MAP_PX - 1, WORLD_MAP_PX - 1
    );
  }

  const sel = state.worldMapDrag
    ? rectFromDrag(state.worldMapDrag)
    : state.worldMapSelection;
  if (sel) {
    const px = sel.minX * WORLD_MAP_PX;
    const py = (WORLD_MAP_H - 1 - sel.maxY) * WORLD_MAP_PX;
    const pw = (sel.maxX - sel.minX + 1) * WORLD_MAP_PX;
    const ph = (sel.maxY - sel.minY + 1) * WORLD_MAP_PX;
    ctx.fillStyle = "rgba(213, 162, 65, 0.25)";
    ctx.fillRect(px, py, pw, ph);
    ctx.strokeStyle = "#d5a241";
    ctx.lineWidth = 2 / state.worldMapZoom;
    ctx.strokeRect(px + 1, py + 1, pw - 2, ph - 2);
  }

  // Paste-ghost preview: each clipboard tile rendered at offset of cursor.
  const anchor = pasteHoverAnchor();
  if (anchor && state.worldMapClipboard) {
    const cb = state.worldMapClipboard;
    const dx = anchor.x - cb.minX;
    const dy = anchor.y - cb.minY;
    const active = new Set(state.info?.regions?.map(r => `${r.x},${r.y}`) || []);
    for (const t of cb.tiles) {
      const tx = t.x + dx;
      const ty = t.y + dy;
      if (tx < 0 || tx > 255 || ty < 0 || ty > 127) continue;
      const occupied = active.has(`${tx},${ty}`);
      ctx.fillStyle = occupied ? "rgba(207, 107, 92, 0.45)" : "rgba(101, 181, 129, 0.5)";
      ctx.fillRect(tx * WORLD_MAP_PX, (WORLD_MAP_H - 1 - ty) * WORLD_MAP_PX, WORLD_MAP_PX, WORLD_MAP_PX);
    }
    // Outline the full bbox so the user sees the shape even on empty terrain.
    const bx = anchor.x * WORLD_MAP_PX;
    const by = (WORLD_MAP_H - 1 - (anchor.y + cb.h - 1)) * WORLD_MAP_PX;
    ctx.strokeStyle = "#65b581";
    ctx.lineWidth = 2 / state.worldMapZoom;
    ctx.strokeRect(bx + 1, by + 1, cb.w * WORLD_MAP_PX - 2, cb.h * WORLD_MAP_PX - 2);
  }

  const cur = cameraRegion();
  ctx.fillStyle = "#e8635c";
  const cx = cur.x * WORLD_MAP_PX + WORLD_MAP_PX / 2;
  const cy = (WORLD_MAP_H - 1 - cur.y) * WORLD_MAP_PX + WORLD_MAP_PX / 2;
  ctx.beginPath();
  ctx.arc(cx, cy, 4 / state.worldMapZoom, 0, Math.PI * 2);
  ctx.fill();

  ctx.setTransform(1, 0, 0, 1, 0, 0);
}

function rectFromDrag(d) {
  return {
    minX: Math.min(d.ax, d.bx),
    maxX: Math.max(d.ax, d.bx),
    minY: Math.min(d.ay, d.by),
    maxY: Math.max(d.ay, d.by)
  };
}

function setWorldMapMode(mode) {
  state.worldMapMode = mode;
  ui.worldMapMode.dataset.mode = mode;
  ui.worldMapMode.textContent = mode === "select" ? "Mode: Edit" : "Mode: Teleport";
  ui.worldMapCanvas.classList.toggle("edit", mode === "select");
  if (mode !== "select") {
    state.worldMapSelection = null;
    state.worldMapDrag = null;
  }
  if (ui.worldMapHint) {
    ui.worldMapHint.textContent = mode === "select"
      ? "Drag to select. Wheel zoom, right-drag pan. Ctrl+C copy · Ctrl+V paste · Esc cancel. Then Create / Delete / Copy."
      : "Click any tile to teleport. Wheel zoom, right-drag pan. Green = active · Yellow = has ref · Orange outline = loaded · Red dot = current position.";
  }
  refreshWorldMapActions();
}

function selectionStats() {
  if (!state.worldMapSelection) return null;
  const s = state.worldMapSelection;
  const active = new Set(state.info?.regions?.map(r => `${r.x},${r.y}`) || []);
  const empties = [];
  const occupied = [];
  for (let yy = s.minY; yy <= s.maxY; yy++) {
    for (let xx = s.minX; xx <= s.maxX; xx++) {
      const key = `${xx},${yy}`;
      if (active.has(key)) occupied.push({ x: xx, y: yy });
      else empties.push({ x: xx, y: yy });
    }
  }
  return { sel: s, empties, occupied };
}

function refreshWorldMapActions() {
  const inEdit = state.worldMapMode === "select";
  const hasSel = !!state.worldMapSelection;
  const hasClipboard = !!state.worldMapClipboard;
  ui.worldMapActions.hidden = !(inEdit && hasSel);
  ui.worldMapPasteBar.hidden = !hasClipboard;
  if (hasClipboard) {
    const cb = state.worldMapClipboard;
    ui.worldMapPasteInfo.textContent =
      `Paste pending: ${cb.tiles.length} region${cb.tiles.length === 1 ? "" : "s"} (${cb.w}×${cb.h}). Click on the map to drop.`;
  }
  if (!hasSel) return;
  const stats = selectionStats();
  const s = stats.sel;
  const w = s.maxX - s.minX + 1;
  const h = s.maxY - s.minY + 1;
  const total = w * h;
  ui.worldMapSelInfo.textContent =
    `Selection ${s.minX},${s.minY} → ${s.maxX},${s.maxY}  (${w}×${h} = ${total} · ${stats.empties.length} empty · ${stats.occupied.length} active)`;
  ui.worldMapCreateBtn.disabled = stats.empties.length === 0;
  ui.worldMapCreateBtn.textContent = stats.empties.length === 0
    ? "Create… (no empty)"
    : `Create ${stats.empties.length} flat region${stats.empties.length === 1 ? "" : "s"}…`;
  ui.worldMapDeleteBtn.disabled = stats.occupied.length === 0;
  ui.worldMapDeleteBtn.textContent = stats.occupied.length === 0
    ? "Delete… (none active)"
    : `Delete ${stats.occupied.length} region${stats.occupied.length === 1 ? "" : "s"}…`;
  ui.worldMapCopyBtn.disabled = stats.occupied.length === 0;
  ui.worldMapCopyBtn.textContent = stats.occupied.length === 0
    ? "Copy (none active)"
    : `Copy ${stats.occupied.length}`;
}

function copyWorldMapSelection() {
  const stats = selectionStats();
  if (!stats || stats.occupied.length === 0) {
    setStatus("Nothing to copy — selection has no active regions");
    return;
  }
  const xs = stats.occupied.map(t => t.x);
  const ys = stats.occupied.map(t => t.y);
  const minX = Math.min(...xs), maxX = Math.max(...xs);
  const minY = Math.min(...ys), maxY = Math.max(...ys);
  state.worldMapClipboard = {
    tiles: stats.occupied,
    minX, minY,
    w: maxX - minX + 1,
    h: maxY - minY + 1
  };
  setStatus(`Copied ${stats.occupied.length} region(s) — move cursor and click on the map to paste, Esc to cancel`);
  refreshWorldMapActions();
}

function clearWorldMapClipboard() {
  if (!state.worldMapClipboard) return;
  state.worldMapClipboard = null;
  setStatus("Paste cancelled");
  refreshWorldMapActions();
}

// pasteHoverAnchor returns the destination top-left {x, y} based on the
// cursor's current region hover. The cursor maps to the source-rect minX/minY.
function pasteHoverAnchor() {
  const cb = state.worldMapClipboard;
  const hover = state.worldMapHover;
  if (!cb || !hover) return null;
  return { x: hover.x, y: hover.y };
}

// pasteCopies maps each source tile to its destination region under the
// optional bundle transform. The transform's rotation also rotates the
// bundle's bounding box, so a 3×2 paste becomes 2×3 at 90°/270°.
function pasteCopies(anchor, transform) {
  const cb = state.worldMapClipboard;
  if (!cb || !anchor) return [];
  const t = transform || { rotation: 0, flipX: false, flipZ: false };
  const W = cb.w, H = cb.h;
  const newW = (t.rotation === 90 || t.rotation === 270) ? H : W;
  const newH = (t.rotation === 90 || t.rotation === 270) ? W : H;
  return cb.tiles.map(tile => {
    let rsx = tile.x - cb.minX;
    let rsy = tile.y - cb.minY;
    // Rotation in bundle space (same convention as the server-side region
    // content transform: CW from above with +Z up on the world map).
    let nrx, nry;
    switch (t.rotation) {
      case 90:  nrx = rsy;            nry = (W - 1) - rsx; break;
      case 180: nrx = (W - 1) - rsx;  nry = (H - 1) - rsy; break;
      case 270: nrx = (H - 1) - rsy;  nry = rsx;           break;
      default:  nrx = rsx;            nry = rsy;
    }
    if (t.flipX) nrx = (newW - 1) - nrx;
    if (t.flipZ) nry = (newH - 1) - nry;
    return {
      srcX: tile.x, srcY: tile.y,
      dstX: anchor.x + nrx,
      dstY: anchor.y + nry
    };
  });
}

function pasteAtHover() {
  if (!state.worldMapClipboard) {
    setStatus("Nothing in clipboard");
    return;
  }
  const anchor = pasteHoverAnchor();
  if (!anchor) {
    setStatus("Hover over a tile, then paste");
    return;
  }
  // Show modal — the actual paste copies are recomputed on confirm using
  // the modal's rotation/flip settings, since those change the bundle layout.
  const cb = state.worldMapClipboard;
  ui.pasteRegionSummary.textContent =
    `Paste ${cb.tiles.length} region${cb.tiles.length === 1 ? "" : "s"} (${cb.w}×${cb.h}) at ${anchor.x},${anchor.y}`;
  ui.pasteRegionFloor.value = "";
  ui.pasteRegionRotation.value = "0";
  ui.pasteRegionFlipX.checked = false;
  ui.pasteRegionFlipZ.checked = false;
  ui.pasteRegionDialog._anchor = anchor;
  ui.pasteRegionDialog.showModal();
  setTimeout(() => ui.pasteRegionFloor.focus(), 0);
}

async function onPasteRegionDialogClose() {
  const dlg = ui.pasteRegionDialog;
  const anchor = dlg._anchor;
  dlg._anchor = null;
  if (dlg.returnValue !== "confirm" || !anchor || !state.worldMapClipboard) return;
  const rotation = parseInt(ui.pasteRegionRotation.value, 10) || 0;
  const flipX = !!ui.pasteRegionFlipX.checked;
  const flipZ = !!ui.pasteRegionFlipZ.checked;
  const copies = pasteCopies(anchor, { rotation, flipX, flipZ });
  if (copies.length === 0) return;
  for (const c of copies) {
    if (c.dstX < 0 || c.dstX > 255 || c.dstY < 0 || c.dstY > 127) {
      setStatus("Paste target out of world bounds (transform pushed it off the map)");
      return;
    }
  }
  const body = { copies, overwrite: true, rotation, flipX, flipZ };
  const raw = ui.pasteRegionFloor.value.trim();
  if (raw !== "") {
    const f = parseFloat(raw);
    if (!isFinite(f)) {
      setStatus("Invalid target floor height");
      return;
    }
    body.targetFloorY = f;
  }
  try {
    const result = await fetchJSON("/api/region/duplicate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    });
    const ok = (result.duplicated || []).length;
    const sk = (result.skipped || []).length;
    const shiftNote = body.targetFloorY != null
      ? ` · floor ${result.sourceFloorY?.toFixed?.(1)} → ${body.targetFloorY} (Δ${result.heightDelta?.toFixed?.(1)})`
      : "";
    setStatus(`Pasted ${ok} region(s)${shiftNote} — saved to disk and export${sk ? ` · ${sk} skipped` : ""}`);
    flashSavedJustNow(`✓ Pasted ${ok}`);
    // Reload any pasted destinations that are within the loaded set so the
    // editor's terrain reflects the new data without a manual recenter. The
    // server now bumps a per-region version counter on every invalidate, so
    // the texture URL it returns differs after a paste and the browser
    // refetches instead of serving its cached copy. Force a lightmap bust
    // here too since loadRegion doesn't pass `bust:true` by default.
    for (const c of result.duplicated || []) {
      const key = keyFor(c.dstX, c.dstY);
      if (state.regions.has(key)) {
        unloadRegion(c.dstX, c.dstY);
        loadRegion(c.dstX, c.dstY, { streaming: true }).then(r => {
          if (r) loadRegionLightmap(r, true);
        });
      }
    }
    try {
      const info = await fetchJSON("/api/info");
      state.info = info;
    } catch (_) {}
    state.worldMapClipboard = null;
    refreshWorldMapActions();
  } catch (err) {
    setStatus(`Paste failed: ${err.message}`);
  }
}

function drawWorldMap() {
  if (!state.worldMapOpen) return;
  renderWorldMap();
}

// Internal canvas pixel under the cursor (independent of CSS sizing).
function worldMapCanvasPixel(evt) {
  const cv = ui.worldMapCanvas;
  const rect = cv.getBoundingClientRect();
  return {
    x: (evt.clientX - rect.left) * cv.width / rect.width,
    y: (evt.clientY - rect.top) * cv.height / rect.height
  };
}

function worldMapPickRegion(evt) {
  const p = worldMapCanvasPixel(evt);
  const px = (p.x - state.worldMapPanX) / state.worldMapZoom;
  const py = (p.y - state.worldMapPanY) / state.worldMapZoom;
  const rx = Math.floor(px / WORLD_MAP_PX);
  const ry = WORLD_MAP_H - 1 - Math.floor(py / WORLD_MAP_PX);
  if (rx < 0 || rx > 255 || ry < 0 || ry > 127) return null;
  return { x: rx, y: ry };
}

function clampWorldMapPan() {
  const cv = ui.worldMapCanvas;
  if (!cv.width || !cv.height) return;
  const mapW = WORLD_MAP_W * WORLD_MAP_PX * state.worldMapZoom;
  const mapH = WORLD_MAP_H * WORLD_MAP_PX * state.worldMapZoom;
  // Keep at least a 32-internal-pixel slice of the map visible.
  const slack = 32;
  state.worldMapPanX = clamp(state.worldMapPanX, slack - mapW, cv.width - slack);
  state.worldMapPanY = clamp(state.worldMapPanY, slack - mapH, cv.height - slack);
}

function onWorldMapWheel(evt) {
  evt.preventDefault();
  const p = worldMapCanvasPixel(evt);
  const worldX = (p.x - state.worldMapPanX) / state.worldMapZoom;
  const worldY = (p.y - state.worldMapPanY) / state.worldMapZoom;
  const factor = evt.deltaY < 0 ? 1.25 : 0.8;
  const newZoom = clamp(state.worldMapZoom * factor, 1, 8);
  if (newZoom === state.worldMapZoom) return;
  state.worldMapZoom = newZoom;
  state.worldMapPanX = p.x - worldX * newZoom;
  state.worldMapPanY = p.y - worldY * newZoom;
  clampWorldMapPan();
}

function onWorldMapMouseMove(evt) {
  if (state.worldMapPanDrag) {
    const p = worldMapCanvasPixel(evt);
    state.worldMapPanX = state.worldMapPanDrag.panX + (p.x - state.worldMapPanDrag.x);
    state.worldMapPanY = state.worldMapPanDrag.panY + (p.y - state.worldMapPanDrag.y);
    clampWorldMapPan();
    return;
  }
  const r = worldMapPickRegion(evt);
  if (!r) return;
  state.worldMapHover = r;
  const meta = state.info?.regions?.find(x => x.x === r.x && x.y === r.y);
  const tag = meta ? (meta.hasRef ? "ref" : "active") : "empty";
  ui.worldMapCoords.textContent = `${r.x},${r.y}  [${tag}]`;
  if (state.worldMapDrag) {
    state.worldMapDrag.bx = r.x;
    state.worldMapDrag.by = r.y;
  }
}

function onWorldMapMouseDown(evt) {
  if (evt.button === 2) {
    evt.preventDefault();
    const p = worldMapCanvasPixel(evt);
    state.worldMapPanDrag = {
      x: p.x, y: p.y,
      panX: state.worldMapPanX, panY: state.worldMapPanY
    };
    return;
  }
  if (evt.button !== 0) return;
  // When a clipboard is active, left-click anywhere = paste; don't start a drag.
  if (state.worldMapClipboard) return;
  const r = worldMapPickRegion(evt);
  if (!r) return;
  if (state.worldMapMode === "select") {
    evt.preventDefault();
    state.worldMapDrag = { ax: r.x, ay: r.y, bx: r.x, by: r.y };
  }
}

function onWorldMapMouseUp(evt) {
  if (!state.worldMapOpen) return;
  if (evt.button === 2 && state.worldMapPanDrag) {
    state.worldMapPanDrag = null;
    return;
  }
  if (evt.button !== 0) return;
  // Clipboard active → click pastes (regardless of mode).
  if (state.worldMapClipboard) {
    const cv = ui.worldMapCanvas;
    if (evt.target !== cv) return;
    pasteAtHover();
    return;
  }
  if (state.worldMapMode === "select" && state.worldMapDrag) {
    state.worldMapSelection = rectFromDrag(state.worldMapDrag);
    state.worldMapDrag = null;
    refreshWorldMapActions();
    return;
  }
  if (state.worldMapMode === "navigate") {
    const cv = ui.worldMapCanvas;
    if (evt.target !== cv) return;
    const r = worldMapPickRegion(evt);
    if (!r) return;
    teleportToRegion(r.x, r.y);
    toggleWorldMap(false);
  }
}

function openCreateRegionDialog() {
  const stats = selectionStats();
  if (!stats || stats.empties.length === 0) return;
  const s = stats.sel;
  ui.createRegionSummary.textContent = `${stats.empties.length} new region${stats.empties.length === 1 ? "" : "s"} in ${s.minX},${s.minY} → ${s.maxX},${s.maxY}`;
  ui.createRegionDialog._pending = stats.empties;
  ui.createRegionDialog.showModal();
}

function openDeleteRegionDialog() {
  const stats = selectionStats();
  if (!stats || stats.occupied.length === 0) return;
  const s = stats.sel;
  ui.deleteRegionSummary.textContent = `Delete ${stats.occupied.length} active region${stats.occupied.length === 1 ? "" : "s"} in ${s.minX},${s.minY} → ${s.maxX},${s.maxY}?`;
  ui.deleteRegionDialog._pending = stats.occupied;
  ui.deleteRegionDialog.showModal();
}

async function onDeleteRegionDialogClose() {
  const dlg = ui.deleteRegionDialog;
  const targets = dlg._pending || [];
  dlg._pending = null;
  if (dlg.returnValue !== "confirm" || targets.length === 0) return;
  try {
    const result = await fetchJSON("/api/region/delete", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ regions: targets })
    });
    const deletedCount = (result.deleted || []).length;
    setStatus(`Deleted ${deletedCount} region(s) — saved to disk and export`);
    flashSavedJustNow(`✓ Deleted ${deletedCount}`);
    for (const r of result.deleted || []) {
      unloadRegion(r.x, r.y);
    }
    try {
      const info = await fetchJSON("/api/info");
      state.info = info;
    } catch (_) {}
    state.worldMapSelection = null;
    refreshWorldMapActions();
    updateMeta();
  } catch (err) {
    setStatus(`Delete failed: ${err.message}`);
  }
}

// unloadRegion drops a region's GPU buffers and placements from the live
// scene without touching disk. Used after server-side delete confirms a
// region is gone, and could be reused for memory eviction later.
function unloadRegion(x, y) {
  const key = keyFor(x, y);
  const region = state.regions.get(key);
  if (region) {
    if (region.texture) gl.deleteTexture(region.texture);
    if (region.lightmap) gl.deleteTexture(region.lightmap);
    if (region.mesh) {
      gl.deleteBuffer(region.mesh.position);
      gl.deleteBuffer(region.mesh.color);
      if (region.mesh.normal) gl.deleteBuffer(region.mesh.normal);
      if (region.mesh.texCoord) gl.deleteBuffer(region.mesh.texCoord);
    }
    if (region.nvmOpenBuffer) gl.deleteBuffer(region.nvmOpenBuffer);
    if (region.nvmClosedBuffer) gl.deleteBuffer(region.nvmClosedBuffer);
    state.regions.delete(key);
  }
  state.dirty.delete(key);
  state.tileDirty.delete(key);
  const regionID = (y << 8) | x;
  const before = state.objectPlacements.length;
  state.objectPlacements = state.objectPlacements.filter(p => p.regionID !== regionID);
  if (state.objectPlacements.length !== before) {
    state.selection = null;
    rebuildObjectMarkers();
    updateSelectionUI();
  }
}

async function onCreateRegionDialogClose() {
  const dlg = ui.createRegionDialog;
  const empties = dlg._pending || [];
  dlg._pending = null;
  if (dlg.returnValue !== "confirm" || empties.length === 0) return;
  const height = parseFloat(ui.createRegionHeight.value);
  if (!isFinite(height)) {
    setStatus("Invalid height");
    return;
  }
  try {
    const result = await fetchJSON("/api/region/create", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ regions: empties, height, defaultTileID: 0 })
    });
    const createdCount = (result.created || []).length;
    setStatus(`Created ${createdCount} region(s) at y=${height} — saved to disk and export`);
    flashSavedJustNow(`✓ Created ${createdCount}`);
    // Refresh info so the world map reflects the new active regions.
    try {
      const info = await fetchJSON("/api/info");
      state.info = info;
    } catch (_) {}
    state.worldMapSelection = null;
    refreshWorldMapActions();
  } catch (err) {
    setStatus(`Create failed: ${err.message}`);
  }
}

function teleportToRegion(rx, ry) {
  // Without a recenter, world coordinates stay anchored to baseX/baseY.
  // For long-distance jumps, recenter base to avoid floating-point loss.
  const dx = Math.abs(rx - state.baseX);
  const dy = Math.abs(ry - state.baseY);
  if (dx > 6 || dy > 6 || state.regions.size === 0) {
    ui.regionX.value = rx;
    ui.regionY.value = ry;
    loadSelectedRegion();
    return;
  }
  // Short hop: just move the camera; streaming will fill in.
  const worldX = regionOffsetX(rx);
  const worldZ = regionOffsetZ(ry);
  state.camera.x = worldX;
  state.camera.z = worldZ;
  // Try to land above the terrain if any nearby region is loaded.
  const h = heightAtWorld(worldX, worldZ)?.height;
  state.camera.y = (h ?? 200) + 650;
  state.lastCamRegion = null; // force stream tick
  setStatus(`Teleported to ${rx},${ry}`);
}

function drawBrushCircle(viewProj) {
  const radius = parseFloat(ui.brushRadius.value);
  const segments = 96;
  const data = new Float32Array(segments * 3);
  for (let i = 0; i < segments; i++) {
    const a = (i / segments) * Math.PI * 2;
    data[i * 3] = state.hover.x + Math.cos(a) * radius;
    data[i * 3 + 1] = state.hover.y + 5;
    data[i * 3 + 2] = state.hover.z + Math.sin(a) * radius;
  }
  gl.bindBuffer(gl.ARRAY_BUFFER, brushBuffer);
  gl.bufferData(gl.ARRAY_BUFFER, data, gl.DYNAMIC_DRAW);
  gl.useProgram(lineProgram.program);
  gl.uniformMatrix4fv(lineProgram.uniforms.uViewProj, false, viewProj);
  const color = state.brushMode === "raise" ? [0.45, 0.95, 0.58] : [0.95, 0.42, 0.34];
  gl.uniform3fv(lineProgram.uniforms.uColor, color);
  bindAttribute(lineProgram.attributes.aPosition, brushBuffer, 3);
  gl.disable(gl.DEPTH_TEST);
  gl.drawArrays(gl.LINE_LOOP, 0, segments);
  gl.enable(gl.DEPTH_TEST);
}

function heightAtWorld(x, z) {
  for (const region of state.regions.values()) {
    const offX = regionOffsetX(region.x);
    const offZ = regionOffsetZ(region.y);
    const lx = offX + 960 - x;
    const lz = z - offZ + 960;
    if (lx < 0 || lx > REGION_SIZE || lz < 0 || lz > REGION_SIZE) continue;
    const gx = clamp(lx / CELL_SIZE, 0, GRID_SIZE - 1);
    const gz = clamp(lz / CELL_SIZE, 0, GRID_SIZE - 1);
    const x0 = Math.floor(gx);
    const z0 = Math.floor(gz);
    const x1 = Math.min(GRID_SIZE - 1, x0 + 1);
    const z1 = Math.min(GRID_SIZE - 1, z0 + 1);
    const fx = gx - x0;
    const fz = gz - z0;
    const h00 = region.heights[z0 * GRID_SIZE + x0];
    const h10 = region.heights[z0 * GRID_SIZE + x1];
    const h01 = region.heights[z1 * GRID_SIZE + x0];
    const h11 = region.heights[z1 * GRID_SIZE + x1];
    const h0 = h00 * (1 - fx) + h10 * fx;
    const h1 = h01 * (1 - fx) + h11 * fx;
    return { height: h0 * (1 - fz) + h1 * fz, region };
  }
  return null;
}

function mouseRay() {
  const ndcX = (state.mouse.x / canvas.clientWidth) * 2 - 1;
  const ndcY = 1 - (state.mouse.y / canvas.clientHeight) * 2;
  const basis = cameraBasis();
  const aspect = canvas.clientWidth / canvas.clientHeight;
  const tan = Math.tan((60 * Math.PI / 180) / 2);
  return normalize([
    basis.forward[0] + basis.right[0] * ndcX * aspect * tan + basis.up[0] * ndcY * tan,
    basis.forward[1] + basis.right[1] * ndcX * aspect * tan + basis.up[1] * ndcY * tan,
    basis.forward[2] + basis.right[2] * ndcX * aspect * tan + basis.up[2] * ndcY * tan
  ]);
}

function cameraBasis() {
  const cp = Math.cos(state.camera.pitch);
  const forward = normalize([
    Math.sin(state.camera.yaw) * cp,
    Math.sin(state.camera.pitch),
    Math.cos(state.camera.yaw) * cp
  ]);
  const right = normalize(cross(forward, [0, 1, 0]));
  const up = normalize(cross(right, forward));
  return { forward, right, up };
}

function viewProjectionMatrix() {
  const basis = cameraBasis();
  const eye = [state.camera.x, state.camera.y, state.camera.z];
  const target = [
    eye[0] + basis.forward[0],
    eye[1] + basis.forward[1],
    eye[2] + basis.forward[2]
  ];
  const view = mat4LookAt(eye, target, basis.up);
  const proj = mat4Perspective(60 * Math.PI / 180, canvas.width / canvas.height, 5, 30000);
  return mat4Multiply(proj, view);
}

function updateMeta() {
  ui.loadedCount.textContent = String(state.regions.size);
  ui.dirtyCount.textContent = String(state.dirty.size);
  ui.objectCount.textContent = String(state.objects.count);
  refreshSaveAllButton();
  const center = state.regions.get(keyFor(state.baseX, state.baseY));
  if (center?.refRegion) {
    ui.refText.textContent = `${center.refRegion.continentName} / ${center.refRegion.areaName}`;
  } else {
    ui.refText.textContent = "None";
  }
}

function setBrushMode(mode) {
  state.brushMode = mode;
  ui.raiseMode.classList.toggle("active", mode === "raise");
  ui.lowerMode.classList.toggle("active", mode === "lower");
  ui.equalizeMode.classList.toggle("active", mode === "equalize");
  ui.tileMode.classList.toggle("active", mode === "tile");
  ui.tilePickerSection.hidden = (mode !== "tile");
  ui.equalizeStatus.hidden = (mode !== "equalize");
  refreshEqualizeStatus();
  if (mode === "tile" && !state.tileCatalog) fetchTileCatalog();
}

function refreshEqualizeStatus() {
  if (!ui.equalizeStatus) return;
  if (state.brushMode !== "equalize") return;
  if (state.equalizeTarget == null) {
    ui.equalizeStatus.textContent = "Middle-click terrain to sample target height";
  } else {
    ui.equalizeStatus.textContent = `Target Y = ${state.equalizeTarget.toFixed(1)} · left-drag to flatten · middle-click to resample`;
  }
}

async function fetchTileCatalog() {
  ui.tileStatus.textContent = "Loading tiles...";
  try {
    const data = await fetchJSON("/api/tiles");
    state.tileCatalog = (data.entries || []).slice().sort((a, b) => a.id - b.id);
    renderTileGrid();
    ui.tileStatus.textContent = `${state.tileCatalog.length} tiles · pick one to paint`;
  } catch (err) {
    ui.tileStatus.textContent = `Failed: ${err.message}`;
  }
}

function renderTileGrid() {
  if (!state.tileCatalog) return;
  const filter = state.tileFilterText.trim().toLowerCase();
  const matches = [];
  for (const e of state.tileCatalog) {
    if (!filter || e.filename.toLowerCase().includes(filter) || e.folder.toLowerCase().includes(filter) || String(e.id) === filter) {
      matches.push(e);
    }
    if (matches.length >= 120) break;
  }
  ui.tileGrid.innerHTML = "";
  for (const e of matches) {
    const cell = document.createElement("div");
    cell.className = "tile-cell";
    if (state.activeTileID === e.id) cell.classList.add("active");
    cell.title = `${e.id} · ${e.folder}/${e.filename}`;
    const img = document.createElement("img");
    img.loading = "lazy";
    img.src = `/api/tile?id=${e.id}`;
    img.alt = "";
    const label = document.createElement("div");
    label.className = "cell-label";
    label.textContent = e.filename.replace(/\.ddj$/i, "");
    cell.appendChild(img);
    cell.appendChild(label);
    cell.addEventListener("click", () => selectActiveTile(e));
    ui.tileGrid.appendChild(cell);
  }
}

function selectActiveTile(entry) {
  state.activeTileID = entry.id;
  ui.tileStatus.textContent = `Active: ${entry.id} · ${entry.folder}/${entry.filename}`;
  for (const cell of ui.tileGrid.querySelectorAll(".tile-cell")) cell.classList.remove("active");
  for (const cell of ui.tileGrid.querySelectorAll(".tile-cell")) {
    if (cell.title.startsWith(`${entry.id} `)) cell.classList.add("active");
  }
}

function updateBrushLabels() {
  ui.radiusOut.textContent = ui.brushRadius.value;
  ui.strengthOut.textContent = ui.brushStrength.value;
}

function setStatus(text) {
  ui.status.textContent = text;
}

function buildTerrainIndices() {
  const indices = new Uint16Array((GRID_SIZE - 1) * (GRID_SIZE - 1) * 6);
  let p = 0;
  for (let z = 0; z < GRID_SIZE - 1; z++) {
    for (let x = 0; x < GRID_SIZE - 1; x++) {
      const a = z * GRID_SIZE + x;
      const b = a + 1;
      const c = a + GRID_SIZE;
      const d = c + 1;
      indices[p++] = a; indices[p++] = c; indices[p++] = b;
      indices[p++] = b; indices[p++] = c; indices[p++] = d;
    }
  }
  gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, terrainIndexBuffer);
  gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, indices, gl.STATIC_DRAW);
  return indices.length;
}

function makeBuffer(data) {
  const buffer = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, data, gl.STATIC_DRAW);
  return buffer;
}

function createWhitePixelTexture() {
  const tex = gl.createTexture();
  gl.bindTexture(gl.TEXTURE_2D, tex);
  gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE,
    new Uint8Array([255, 255, 255, 255]));
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  return tex;
}

function bindAttribute(location, buffer, size) {
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.enableVertexAttribArray(location);
  gl.vertexAttribPointer(location, size, gl.FLOAT, false, 0, 0);
}

function bindAttributeStride(location, buffer, size, stride, offset) {
  if (location === undefined || location < 0) return;
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.enableVertexAttribArray(location);
  gl.vertexAttribPointer(location, size, gl.FLOAT, false, stride, offset);
}

function createProgram(vsSource, fsSource) {
  const vs = compileShader(gl.VERTEX_SHADER, vsSource);
  const fs = compileShader(gl.FRAGMENT_SHADER, fsSource);
  const program = gl.createProgram();
  gl.attachShader(program, vs);
  gl.attachShader(program, fs);
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    throw new Error(gl.getProgramInfoLog(program));
  }
  const attributes = {};
  const uniforms = {};
  const attrCount = gl.getProgramParameter(program, gl.ACTIVE_ATTRIBUTES);
  for (let i = 0; i < attrCount; i++) {
    const info = gl.getActiveAttrib(program, i);
    attributes[info.name] = gl.getAttribLocation(program, info.name);
  }
  const uniformCount = gl.getProgramParameter(program, gl.ACTIVE_UNIFORMS);
  for (let i = 0; i < uniformCount; i++) {
    const info = gl.getActiveUniform(program, i);
    uniforms[info.name] = gl.getUniformLocation(program, info.name);
  }
  return { program, attributes, uniforms };
}

function compileShader(type, source) {
  const shader = gl.createShader(type);
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    throw new Error(gl.getShaderInfoLog(shader));
  }
  return shader;
}

async function fetchJSON(url, options) {
  const res = await fetch(url, options);
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) throw new Error(data?.error || res.statusText);
  return data;
}

function resize() {
  const dpr = Math.min(2, window.devicePixelRatio || 1);
  const w = Math.floor(canvas.clientWidth * dpr);
  const h = Math.floor(canvas.clientHeight * dpr);
  if (canvas.width !== w || canvas.height !== h) {
    canvas.width = w;
    canvas.height = h;
  }
}

function regionOffsetX(x) {
  return -(x - state.baseX) * REGION_SIZE;
}

function regionOffsetZ(y) {
  return (y - state.baseY) * REGION_SIZE;
}

function keyFor(x, y) {
  return `${x},${y}`;
}

function isTyping(target) {
  return target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);
}

function clamp(v, lo, hi) {
  return Math.max(lo, Math.min(hi, v));
}

function normalize(v) {
  const len = Math.hypot(v[0], v[1], v[2]) || 1;
  return [v[0] / len, v[1] / len, v[2] / len];
}

function cross(a, b) {
  return [
    a[1] * b[2] - a[2] * b[1],
    a[2] * b[0] - a[0] * b[2],
    a[0] * b[1] - a[1] * b[0]
  ];
}

function dot(a, b) {
  return a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
}

function mat4Perspective(fovy, aspect, near, far) {
  const f = 1 / Math.tan(fovy / 2);
  const out = new Float32Array(16);
  out[0] = f / aspect;
  out[5] = f;
  out[10] = (far + near) / (near - far);
  out[11] = -1;
  out[14] = (2 * far * near) / (near - far);
  return out;
}

function mat4LookAt(eye, target, up) {
  const z = normalize([eye[0] - target[0], eye[1] - target[1], eye[2] - target[2]]);
  const x = normalize(cross(up, z));
  const y = cross(z, x);
  const out = new Float32Array(16);
  out[0] = x[0]; out[1] = y[0]; out[2] = z[0]; out[3] = 0;
  out[4] = x[1]; out[5] = y[1]; out[6] = z[1]; out[7] = 0;
  out[8] = x[2]; out[9] = y[2]; out[10] = z[2]; out[11] = 0;
  out[12] = -dot(x, eye); out[13] = -dot(y, eye); out[14] = -dot(z, eye); out[15] = 1;
  return out;
}

function mat4Multiply(a, b, out) {
  const r = out || new Float32Array(16);
  for (let col = 0; col < 4; col++) {
    for (let row = 0; row < 4; row++) {
      r[col * 4 + row] =
        a[0 * 4 + row] * b[col * 4 + 0] +
        a[1 * 4 + row] * b[col * 4 + 1] +
        a[2 * 4 + row] * b[col * 4 + 2] +
        a[3 * 4 + row] * b[col * 4 + 3];
    }
  }
  return r;
}
