// The top of the sidebar (ADR-0023): Andon's lantern and name, then Herdr's search and sidebar buttons.
//
// A changed copy of the brand in Herdr's shipped `workspace-chrome` mod (herdr-app, defaults/mods/
// workspace-chrome/main.js at commit 903dc810da0c839ba865d6b190f3aeeceb3a636b), by the herdr-app
// authors, under the Apache License 2.0 (LICENSE beside this file). Changed by Andon: the lantern
// and "Andon" in place of Herdr's logo and name; the footer is left to workspace-chrome.
// Everything else is Herdr's code as shipped.
import { div, svg } from "gpui-kit";
import { h_flex } from "gpui-base";
import { theme, run, window, shared, setShared, layout } from "herdr";

// The logo's window colors (docs/assets/logo), as the inbox lights its rows (../inbox/main.js GLOW).
export const GLOW = { amber: { lamp: "#F0A202", hi: "#FFD36B" }, red: { lamp: "#D8453B", hi: "#F29A92" } };
// Set by the inbox: red, amber, or null when nothing needs you (unlit).
const LANTERN = "inbox.lantern";
// How long the window takes to change color. It never blinks.
const FADE = { duration: 900, easing: "ease-in-out" };

function find(node, id) {
  if (node?.id === id) return node;
  for (const child of node?.children ?? []) {
    const found = find(child, id);
    if (found) return found;
  }
  return null;
}

// The sidebar's rail goes by its width, not a flag: dragging its edge narrows it too.
const narrow = () => (find(layout(), "sidebar")?.size?.px ?? Infinity) < 160;

// To the rail (on macOS wide enough for the traffic lights), or back to the width it had.
function toggleSidebar() {
  const width = find(layout(), "sidebar")?.size?.px;
  if (!narrow()) setShared("sidebar.width", width);
  // Mods edited before the rail went by width still read the flag.
  setShared("sidebar.collapsed", !narrow());
  run("layout.resize", { id: "sidebar", px: narrow() ? (shared("sidebar.width") ?? 268) : window()?.platform === "macos" ? 76 : 44 });
}

function button(t, id, label, action) {
  return div().id(id).size(26).flex_none().flex().items_center().justify_center().rounded(6).text_color(t.muted).hover(s => s.bg(t.hover).text_color(t.foreground))
    .on_mouse_down("left", (_e, cx) => cx.stop_propagation()).on_click(action).child(label);
}

// The logo drawn in layers, since svg() paints one color: the frame, the unlit window, then each
// light over it, shown or faded out.
function lantern(t, lit) {
  const layer = (file, color) => svg(`mods/andon/${file}`).size(20).absolute().top(0).left(0).text_color(color);
  const light = (name) => div().id(`lantern-${name}`).absolute().top(0).left(0).size(20)
    .opacity(lit === name ? 1 : 0).transition("opacity", FADE)
    .child(layer("lantern-window.svg", GLOW[name].lamp)).child(layer("lantern-glow.svg", GLOW[name].hi));
  return div().id("lantern").relative().size(20).flex_none()
    .child(layer("lantern-frame.svg", t.foreground))
    .child(layer("lantern-window.svg", t.dim))
    .children(Object.keys(GLOW).map(light));
}

export function brand() {
  const t = theme();
  const collapsed = narrow();
  const lit = shared(LANTERN) ?? null;
  return h_flex().size_full().bg(t.chrome).pl(collapsed ? 6 : 14).pr(10).gap(4).items_center().text_size(13.5)
    .when(window()?.platform === "macos" && !window()?.fullscreen && !collapsed, el => el.pl(76))
    // On the rail there is no room: the lantern goes with the name (the inbox's rows still light).
    .when(!collapsed, el => el.child(h_flex().flex_1().gap(8).items_center().child(lantern(t, lit)).child(div().font_semibold().text_color(t.foreground).child("Andon"))))
    .when(!collapsed, el => el.child(button(t, "search", svg("mods/workspace-chrome/search.svg").size(15), () => run("mod.open", { name: "picker" }))))
    // On the macOS rail the traffic lights fill the row; the agents list's » unfolds it.
    .when(!(collapsed && window()?.platform === "macos"), el => el.child(button(t, "sidebar-toggle", svg("mods/workspace-chrome/sidebar.svg").size(15), toggleSidebar)));
}
