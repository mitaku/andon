import { View, div } from "gpui-kit";
import { terminals, panes, spaces, run, props } from "herdr";
import { brand } from "./brand.js";

// Read the actual pane placement: labels contain machine-local ids, while
// spaces/panes use qualified ids for remote machines. Never create a target.
// A copy with props {brand: true} draws the top of the sidebar instead and names nothing.
export default class Andon extends View {
  render() {
    if (props()?.brand) return brand();
    const all = new Map(terminals().map(t => [t.address, t]));
    const tabs = new Map();
    for (const space of spaces()) {
      for (const tab of space.tabs) tabs.set(tab.id, { space, tab });
    }
    const commands = new Map();
    const rename = (kind, target, name) => {
      if (target.name !== name) {
        commands.set(`${kind}:${target.id}`, [kind + ".rename", { id: target.id, name }]);
      }
    };
    for (const pane of panes()) {
      const terminal = all.get(pane.terminal);
      const location = tabs.get(pane.tab);
      if (!terminal || terminal.state !== "live" || !location) continue;
      const labels = terminal.labels ?? {};
      const labelSpace = labels["herdr.space"];
      const { space, tab } = location;
      const prefix = space.machine ? `${space.machine}/` : "";
      if (space.id !== prefix + labelSpace) continue;
      if (labels.role === "dispatcher") {
        // The dispatcher is one per cell and spans projects, so it has no project
        // label: the name comes from its own ids. One ledger: the project's
        // command-side Space (andon-<x>). Else the cell's Space (andon-<x>-plant).
        const m = /^andon-(.+)-dispatcher$/.exec(tab.id.slice(prefix.length));
        if (!m) continue;
        if (labelSpace === `andon-${m[1]}`) rename("space", space, m[1]);
        else if (labelSpace === `andon-${m[1]}-plant`) rename("space", space, `${m[1]} plant`);
        else continue;
        rename("tab", tab, "dispatcher");
        continue;
      }
      const slug = labels.project;
      if (typeof slug !== "string" || !slug) continue;
      // Command side: one Space per project (CoS and CE tabs). Work side: one
      // Space per issue, its tab holds the worker (ADR-0003). Reviewers share
      // the worker's Space and tab with their own issue/title, so their labels
      // never match the shapes below and they rename nothing.
      const title = typeof labels.title === "string"
        ? labels.title.replace(/[\s\p{Cc}]+/gu, " ").trim() : "";
      const issue = typeof labels.issue === "string" ? labels.issue : "";
      let spaceName, tabKey, tabName;
      if (labelSpace === `andon-${slug}`) {
        spaceName = slug;
        if (labels.role === "chief-of-staff") [tabKey, tabName] = ["cos", "CoS"];
        else if (labels.role === "chief-engineer" && issue && title) [tabKey, tabName] = [issue, `CE ${issue} ${title}`];
        else continue;
      } else if (!labels.role && issue && title && labelSpace === `andon-${slug}-${issue}`) {
        [spaceName, tabKey, tabName] = [`${issue} ${title}`, issue, issue];
      } else continue;
      if (tab.id !== `${prefix}andon-${slug}-${tabKey}`) continue;
      rename("space", space, spaceName);
      rename("tab", tab, tabName);
    }
    for (const [command, args] of commands.values()) run(command, args);
    return div();
  }
}
