/**
 * The layout detector, run inside the page (Playwright's `page.evaluate`), so it
 * must stay one self-contained function: no imports, no closure over this module.
 *
 * It is the 19 Nov 2026 staging audit's detector (648 page runs), made strict
 * enough to gate a merge. Five findings, each a defect jsdom cannot see because
 * it never lays anything out:
 *
 *   overlap   two visible pieces of text (or controls) cross by more than 2px on
 *             both axes. Leaf text is measured by its ink, clipped to every box
 *             that clips it, so a truncated name does not count.
 *   clipped   text pokes more than 2px past the content box of an element that
 *             does not scroll and is not a `truncate`/ellipsis or line clamp.
 *             A negative margin is a deliberate bleed (a hover wash), not a spill.
 *   heading   a column heading more than 4px off its cells. Left-aligned: the
 *             heading's text against the cell's column box (`data-col`), so an
 *             avatar, a status dot or a chip's padding inside the column is the
 *             column's own (D-1 part 2 frame 10 draws Person over the avatar,
 *             Status over the dot). Right-aligned: text against text.
 *   seat      from tablet up, a row's action seat off its first line although the
 *             row's parts fit on one line at their natural widths (D-1 §4 rule 1).
 *   steady    an unchosen FilterPills or Tabs option whose `label-steady` is
 *             narrower than its words at the chosen weight, so choosing it would
 *             push its neighbours (20 Nov 2026). Read only: it measures, never clicks.
 *   edge      something drawn like a card inside the page (a frame's radius or
 *             more, a one-pixel border, a surface token's ground) that does not wear
 *             the light under your hand (A-25 moment 2; owner, 20 Nov 2026: "this
 *             element is not responding to the hover"). Its wearers are a button's
 *             `rim` and the frame utilities `card` and `panel`; what is inside
 *             `[data-no-hand]`, a dialog or a popover is out of it by design.
 *   lamp      the mark's breathing glow (A-25 moment 3) reaching past a box that
 *             clips it (the rail's scroller, the phone's bar, the window): its pool
 *             must be spent inside every edge, read from the glow's own computed box.
 *
 * Page-level horizontal scroll is reported too. `tablet` is the breakpoint below
 * which the seat wraps by design (D-1 §4 rule 9).
 */
export function detect({ tablet = 720 } = {}) {
  const R = (r) => ({ x: Math.round(r.left), y: Math.round(r.top + scrollY), w: Math.round(r.right - r.left), h: Math.round(r.bottom - r.top) });
  // `drawn`: on screen, read aloud or not. A heading lines up with what is drawn under it, an
  // ArrowLink's aria-hidden arrow included; the other checks count only what a reader is given.
  const vis = (el, drawn = false) => {
    if (!el.isConnected) return false;
    if (el.closest(drawn ? "[hidden]" : '[aria-hidden="true"],[hidden],[inert]')) return false;
    const cs = getComputedStyle(el);
    if (cs.visibility !== "visible" || cs.display === "none") return false;
    for (let a = el; a && a !== document.documentElement; a = a.parentElement) {
      const s = getComputedStyle(a);
      if (parseFloat(s.opacity) === 0) return false;
      if (s.clip && s.clip !== "auto") return false;
      if (s.clipPath && s.clipPath.startsWith("inset(50%")) return false;
    }
    const r = el.getBoundingClientRect();
    return r.width > 1 && r.height > 1;
  };
  const sel = (el) => {
    let s = el.tagName.toLowerCase();
    const c = (el.getAttribute("class") || "").split(/\s+/).filter(Boolean).slice(0, 4);
    if (c.length) s += "." + c.join(".");
    const p = el.parentElement;
    const pc = p ? p.tagName.toLowerCase() + (p.getAttribute("class") || "").split(/\s+/).filter(Boolean).slice(0, 3).map((x) => "." + x).join("") : "";
    return (pc ? pc + " > " : "") + s;
  };
  const txt = (el) => (el.innerText || el.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60);
  const clipRect = (el) => {
    let r = { l: -1e9, t: -1e9, r: 1e9, b: 1e9 };
    for (let a = el; a && a !== document.body; a = a.parentElement) {
      const s = getComputedStyle(a);
      if (s.overflowX !== "visible" || s.overflowY !== "visible") {
        const b = a.getBoundingClientRect();
        r = { l: Math.max(r.l, b.left), t: Math.max(r.t, b.top), r: Math.min(r.r, b.right), b: Math.min(r.b, b.bottom) };
      }
    }
    return r;
  };
  const textRects = (el, own) => {
    const out = [];
    const nodes = own ? [...el.childNodes].filter((n) => n.nodeType === 3) : [];
    if (!own) {
      const tw = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
      let n;
      while ((n = tw.nextNode())) if (n.parentElement && vis(n.parentElement, true)) nodes.push(n);
    }
    for (const n of nodes) {
      if (!n.textContent.trim()) continue;
      const rg = document.createRange();
      rg.selectNodeContents(n);
      for (const q of rg.getClientRects()) if (q.width >= 1) out.push(q);
    }
    return out;
  };
  const union = (rects) => ({
    left: Math.min(...rects.map((q) => q.left)),
    top: Math.min(...rects.map((q) => q.top)),
    right: Math.max(...rects.map((q) => q.right)),
    bottom: Math.max(...rects.map((q) => q.bottom)),
  });
  // A box, less any negative margin: `-mx-2` on a link is a hover wash bleeding past the row on
  // purpose, and the link's place is where its margins put it.
  const layoutBox = (el) => {
    const b = el.getBoundingClientRect();
    const s = getComputedStyle(el);
    const m = (v) => Math.max(0, -parseFloat(v) || 0);
    return { left: b.left + m(s.marginLeft), right: b.right - m(s.marginRight), top: b.top + m(s.marginTop), bottom: b.bottom - m(s.marginBottom) };
  };
  const BOXY = 'button,a[href],[role=button],[role=tab],[data-chip],[data-badge],[class*="chip"],[class*="badge"],[class*="pill"],input,select,textarea';
  // A control's box is its reach, so a 44px tap area counts in an overlap; in the clipped check its
  // place is its layout box, less a deliberate bleed.
  const inkOf = (el, boxy, bleed = false) => {
    if (boxy) return bleed ? layoutBox(el) : el.getBoundingClientRect();
    const rects = textRects(el, true);
    return rects.length ? union(rects) : el.getBoundingClientRect();
  };
  const layer = (el) => {
    for (let a = el; a; a = a.parentElement) {
      const p = getComputedStyle(a).position;
      if (p === "fixed" || p === "sticky") return a;
    }
    return null;
  };

  // ── overlap ──
  const cand = [];
  for (const el of document.body.querySelectorAll("*")) {
    if (["SCRIPT", "STYLE", "NOSCRIPT", "svg", "path", "TEMPLATE"].includes(el.tagName)) continue;
    const boxy = el.matches(BOXY);
    if (!boxy && ![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
    if (!vis(el)) continue;
    // An inline link that wraps is one box per line, not the rectangle around both lines: that
    // rectangle covered the words beside it in every sentence it wrapped in.
    const inks = boxy && getComputedStyle(el).display === "inline" ? [...el.getClientRects()] : [inkOf(el, boxy)];
    const c = clipRect(el);
    for (const ink of inks) {
      const r = { left: Math.max(ink.left, c.l), top: Math.max(ink.top, c.t), right: Math.min(ink.right, c.r), bottom: Math.min(ink.bottom, c.b) };
      if (r.right - r.left < 2 || r.bottom - r.top < 2) continue;
      cand.push({ el, r, fx: layer(el) });
    }
  }
  const overlap = [];
  for (let i = 0; i < cand.length; i++) {
    for (let j = i + 1; j < cand.length; j++) {
      const a = cand[i];
      const b = cand[j];
      if (a.el.contains(b.el) || b.el.contains(a.el) || a.fx !== b.fx) continue;
      const ox = Math.min(a.r.right, b.r.right) - Math.max(a.r.left, b.r.left);
      const oy = Math.min(a.r.bottom, b.r.bottom) - Math.max(a.r.top, b.r.top);
      if (ox > 2 && oy > 2) overlap.push(`${Math.round(ox)}x${Math.round(oy)}px: "${txt(a.el)}" (${sel(a.el)}) over "${txt(b.el)}" (${sel(b.el)}) at ${JSON.stringify(R(a.r))}`);
    }
  }

  // ── clipped ──
  const docW = document.documentElement.scrollWidth;
  const vw = document.documentElement.clientWidth;
  const pageScroll = docW > vw + 1 ? [`the page scrolls sideways: ${docW}px in ${vw}px`] : [];
  const clipped = new Map();
  for (const el of document.body.querySelectorAll("*")) {
    if (el.scrollWidth <= el.clientWidth + 2 || el.clientWidth === 0) continue;
    if (["svg", "path", "INPUT", "TEXTAREA", "SELECT", "HTML", "BODY"].includes(el.tagName)) continue;
    const cs = getComputedStyle(el);
    if (cs.display === "inline" || cs.overflowX === "auto" || cs.overflowX === "scroll") continue;
    if (cs.textOverflow === "ellipsis" || (cs.webkitLineClamp && cs.webkitLineClamp !== "none")) continue;
    if (!vis(el)) continue;
    const b = el.getBoundingClientRect();
    const inner = { l: b.left + el.clientLeft + parseFloat(cs.paddingLeft), r: b.left + el.clientLeft + el.clientWidth - parseFloat(cs.paddingRight) };
    let worst = null;
    for (const d of el.querySelectorAll("*")) {
      const ds = getComputedStyle(d);
      if (ds.position === "absolute" || ds.position === "fixed") continue;
      let skip = false;
      for (let a = d.parentElement; a && a !== el; a = a.parentElement) {
        const as = getComputedStyle(a);
        if (as.position === "absolute" || as.position === "fixed" || as.overflowX !== "visible") {
          skip = true;
          break;
        }
      }
      if (skip) continue;
      const boxy = d.matches(BOXY);
      if (!boxy && ![...d.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
      const ink = inkOf(d, boxy, true);
      const spill = Math.max(ink.right - inner.r, inner.l - ink.left);
      if (spill > 2 && vis(d) && (!worst || spill > worst.spill)) worst = { spill: Math.round(spill), el: d };
    }
    if (!worst) continue;
    const prev = clipped.get(worst.el);
    if (!prev || prev.container.contains(el)) {
      clipped.set(worst.el, { container: el, line: `${worst.spill}px: "${txt(worst.el)}" (${sel(worst.el)}) past ${sel(el)} (${el.clientWidth}px, needs ${el.scrollWidth}px)` });
    }
  }

  // ── heading ──
  const cellInk = (cell) => {
    const out = textRects(cell, false);
    for (const e of cell.querySelectorAll("svg,img,button,input,[class*=chip],[class*=badge]")) if (vis(e, true)) out.push(e.getBoundingClientRect());
    if (!out.length) return null;
    const top = Math.min(...out.map((q) => q.top));
    const line = out.filter((q) => q.top < top + 6);
    return { left: Math.min(...out.map((q) => q.left)), right: Math.max(...line.map((q) => q.right)) };
  };
  const rightAligned = (el) => {
    const cs = getComputedStyle(el);
    return cs.textAlign === "right" || cs.textAlign === "end" || (cs.display.includes("flex") && /end/.test(cs.justifyContent));
  };
  const parts = (line) => [...line.children].filter((c) => !c.matches("[data-row-detail]"));
  const heading = [];
  const heads = [...document.querySelectorAll("[data-card-columns], [role=row]")].filter(
    (h) => vis(h) && (h.matches("[data-card-columns]") || h.querySelector(":scope > [role=columnheader]")),
  );
  for (const h of heads) {
    const card = h.closest('[role="table"]') || h.closest(".card") || h.parentElement;
    const lines = new Set([...card.querySelectorAll("[data-card-row], .row-cols, [role=row]"), ...[...card.querySelectorAll("[data-col]")].map((c) => c.parentElement)]);
    const rows = [...lines]
      .filter((r) => r !== h && !heads.includes(r) && !h.contains(r) && !r.contains(h) && vis(r))
      .filter((r) => h.compareDocumentPosition(r) & Node.DOCUMENT_POSITION_FOLLOWING)
      .slice(0, 25);
    parts(h).forEach((hc, i) => {
      if (!vis(hc)) return;
      const seatHead = hc.matches(".row-seat");
      const hInk = cellInk(hc);
      if (!hInk) return;
      const right = rightAligned(hc);
      for (const r of rows) {
        const cells = parts(r);
        const cell = seatHead ? cells.find((c) => c.matches("[data-action-seat], .row-seat")) : cells[i];
        if (!cell || !vis(cell)) continue;
        let d;
        if (right) {
          const ci = cellInk(cell);
          if (!ci) continue;
          d = ci.right - hInk.right;
        } else if (cell.matches("[data-col]") && hc.matches("[data-col]")) {
          d = cell.getBoundingClientRect().left - hInk.left;
        } else {
          const ci = cellInk(cell);
          if (!ci) continue;
          d = ci.left - hInk.left;
        }
        if (Math.abs(d) > 4) {
          heading.push(`"${txt(hc)}" ${Math.round(d)}px off its cell "${txt(cell).slice(0, 30)}" (${right ? "right" : "left"} edge; ${sel(hc)})`);
          break;
        }
      }
    });
  }

  // A sort arrow stays inside its heading's column, pressed or not (it is hung, so nothing else
  // keeps it there): past a right-aligned heading it stood 14px beyond the dates under Found.
  for (const arrow of document.querySelectorAll("[role=columnheader] button > [aria-hidden]")) {
    const col = arrow.closest("[data-col]");
    if (!col || !col.closest("[data-card-columns]") || getComputedStyle(col).display === "none") continue;
    if (!vis(col.closest("[data-card-columns]"))) continue;
    const a = arrow.getBoundingClientRect();
    const c = col.getBoundingClientRect();
    const out = Math.max(c.left - a.left, a.right - c.right);
    if (out > 1) heading.push(`the sort arrow of "${txt(col)}" ${Math.round(out)}px outside its column (${sel(col)})`);
  }

  // ── seat ──
  const seat = [];
  if (innerWidth >= tablet) {
    for (const row of document.querySelectorAll("[data-card-row], .row-cols")) {
      if (!vis(row) || row.matches("[data-card-columns]")) continue;
      const s = [...row.children].find((c) => c.matches("[data-action-seat], .row-seat"));
      if (!s || !vis(s) || !s.innerText.trim()) continue;
      const kids = [...row.children].filter((c) => vis(c) && !c.matches("[data-row-detail]"));
      const first = kids[0];
      if (!first || first === s) continue;
      const fr = first.getBoundingClientRect();
      const sr = s.getBoundingClientRect();
      if (sr.top < fr.bottom - 2 && sr.bottom > fr.top + 2) continue;
      const cs = getComputedStyle(row);
      const avail = row.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) - parseFloat(cs.borderLeftWidth);
      // A row whose first part takes the whole line is stacked by design (a person's role row
      // below its four columns, D-1 §4 rule 9 on a phone): its seat has no first line to share.
      if (fr.width >= avail - 1) continue;
      // What each part needs: a fixed seat (`flex: none`: a number, a status, the action seat) its
      // own width, which D-1 gives it whatever it holds; a share its text on one line, held to its
      // min-width, the narrowest the table lets it be before the row wraps.
      let need = (parseFloat(cs.columnGap) || 0) * (kids.length - 1);
      for (const k of kids) {
        const ks = getComputedStyle(k);
        if (ks.flexGrow === "0" && ks.flexShrink === "0") {
          need += k.getBoundingClientRect().width;
          continue;
        }
        const old = k.style.cssText;
        k.style.width = "max-content";
        k.style.flex = "none";
        k.style.maxWidth = "none";
        need += k.getBoundingClientRect().width;
        k.style.cssText = old;
      }
      if (need <= avail) seat.push(`"${txt(s)}" off the first line of "${txt(row)}" although it fits (${Math.round(need)}px of ${Math.round(avail)}px)`);
    }
  }

  // ── steady ──
  // An unchosen option of FilterPills or Tabs is already as wide as it will be once chosen and
  // semibold, so choosing it moves nothing (owner, 20 Nov 2026). Its `label-steady` must be at least
  // as wide as its words at the chosen option's weight, measured in a detached copy; 0.5px of rounding.
  const steady = [];
  for (const label of document.querySelectorAll(".label-steady")) {
    const option = label.closest("[role=radio], button, a");
    if (!option || !vis(label) || option.matches('[aria-checked="true"], [aria-current]')) continue;
    const words = label.firstElementChild;
    if (!words) continue;
    if (label.dataset.label !== words.textContent) {
      steady.push(`"${txt(option)}" reserves "${label.dataset.label}", not its own words (${sel(option)})`);
      continue;
    }
    const chosen = option.parentElement?.querySelector('[aria-checked="true"], [aria-current]');
    const probe = document.createElement("span");
    const ws = getComputedStyle(words);
    probe.style.cssText = "position:absolute;visibility:hidden;white-space:pre;left:-9999px;top:0";
    for (const k of ["fontFamily", "fontSize", "fontStyle", "fontStretch", "letterSpacing", "fontFeatureSettings"]) probe.style[k] = ws[k];
    probe.style.fontWeight = chosen ? getComputedStyle(chosen).fontWeight : "600";
    probe.textContent = words.textContent;
    document.body.appendChild(probe);
    const need = probe.getBoundingClientRect().width;
    probe.remove();
    const have = label.getBoundingClientRect().width;
    if (have + 0.5 < need) steady.push(`"${txt(option)}" is ${have.toFixed(1)}px, ${need.toFixed(1)}px once chosen: choosing it moves its neighbours (${sel(option)})`);
  }

  // ── edge ──
  // A card by how it is drawn, not by its class: what grep cannot find (a frame typed by hand).
  // The frame radii, the pill's and the surface grounds are read from the tokens themselves.
  const edge = [];
  const page = document.querySelector("main[data-hand]");
  if (page) {
    const token = (prop, value) => {
      const probe = document.createElement("div");
      probe.style[prop] = value;
      document.body.appendChild(probe);
      const got = getComputedStyle(probe)[prop];
      probe.remove();
      return got;
    };
    // A colour as sRGB and alpha, whatever syntax it is computed in (rgb, oklab, color-mix): painted
    // onto one pixel and read back. A surface token at any strength is still that surface.
    const ink = document.createElement("canvas").getContext("2d", { willReadFrequently: true });
    const paint = (colour) => {
      ink.clearRect(0, 0, 1, 1);
      ink.fillStyle = "rgba(0, 0, 0, 0)";
      ink.fillStyle = colour;
      ink.fillRect(0, 0, 1, 1);
      return [...ink.getImageData(0, 0, 1, 1).data];
    };
    const grounds = ["--surface-0", "--surface-1", "--surface-2"].map((t) => paint(token("backgroundColor", `var(${t})`)));
    const isGround = (colour) => {
      const [r, g, b, a] = paint(colour);
      return a > 0 && grounds.some(([R, G, B]) => Math.abs(r - R) <= 4 && Math.abs(g - G) <= 4 && Math.abs(b - B) <= 4);
    };
    const radius = Math.min(parseFloat(token("borderTopLeftRadius", "var(--radius-card)")), parseFloat(token("borderTopLeftRadius", "var(--radius-panel)")));
    const pill = parseFloat(token("borderTopLeftRadius", "var(--radius-pill)"));
    const CORNERS = ["borderTopLeftRadius", "borderTopRightRadius", "borderBottomRightRadius", "borderBottomLeftRadius"];
    const SIDES = ["borderTopWidth", "borderRightWidth", "borderBottomWidth", "borderLeftWidth"];
    for (const el of page.querySelectorAll("*")) {
      const cs = getComputedStyle(el);
      // A frame's corners: as round as a card or a panel, never a pill (a pill is a control, not a card).
      const round = Math.max(...CORNERS.map((c) => parseFloat(cs[c]) || 0));
      if (round < radius || round >= pill) continue;
      // A hairline all round: every side drawn, none heavier than a hairline.
      if (!SIDES.every((side) => { const w = parseFloat(cs[side]); return w > 0 && w <= 1.5; })) continue;
      // On a surface: a surface token at any alpha, or a drawn ground (a gradient) on a frame's shape.
      const onSurface = isGround(cs.backgroundColor) || cs.backgroundImage !== "none";
      if (!onSurface) continue;
      if (el.matches(".rim, .card, .panel")) continue;
      if (el.closest('[data-no-hand], [role="dialog"], [popover], .shadow-popover')) continue;
      if (!vis(el)) continue;
      edge.push(`"${txt(el)}" is drawn like a card and does not wear the edge (${sel(el)})`);
    }
  }

  // ── lamp ──
  // The glow behind a lit mark is a closest-side pool, so it is spent at its own box's edge: that box
  // (the mark's, grown by the pseudo-element's computed inset) must sit inside everything that clips it.
  const lamp = [];
  for (const mark of document.querySelectorAll(".syndra-mark[data-room]")) {
    // Drawn, read aloud or not: the mark is aria-hidden, and its light is what is on screen.
    if (!vis(mark, true)) continue;
    const glow = getComputedStyle(mark, "::before");
    if (glow.content === "none" || glow.display === "none") continue;
    const m = mark.getBoundingClientRect();
    const box = { l: m.left + parseFloat(glow.left), t: m.top + parseFloat(glow.top), r: m.right - parseFloat(glow.right), b: m.bottom - parseFloat(glow.bottom) };
    const clip = clipRect(mark);
    const edge = { l: Math.max(clip.l, 0), t: Math.max(clip.t, 0), r: Math.min(clip.r, innerWidth), b: Math.min(clip.b, innerHeight) };
    const past = [
      ["left", edge.l - box.l],
      ["top", edge.t - box.t],
      ["right", box.r - edge.r],
      ["bottom", box.b - edge.b],
    ].filter(([, by]) => by > 0.5);
    for (const [side, by] of past) lamp.push(`the lamp's glow is cut ${by.toFixed(1)}px at its ${side} (${sel(mark)})`);
  }

  return { overlap, clipped: [...clipped.values()].map((v) => v.line), heading, seat, steady, edge, lamp, pageScroll };
}
