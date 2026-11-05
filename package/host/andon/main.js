import { View, div } from "gpui-kit";
import { terminals, panes, spaces, run } from "herdr";

// Read the actual pane placement: labels contain machine-local ids, while
// spaces/panes use qualified ids for remote machines. Never create a target.
export default class Andon extends View {
  render() {
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
      const slug = labels.project;
      if (typeof slug !== "string" || !slug) continue;
      const labelSpace = labels["herdr.space"];
      const work = labelSpace === `andon-${slug}-work`;
      if (!work && labelSpace !== `andon-${slug}`) continue;
      const { space, tab } = location;
      const prefix = space.machine ? `${space.machine}/` : "";
      if (space.id !== prefix + labelSpace) continue;
      // Reviewers share the worker's tab, but their own task/title must not
      // replace its name. Only the three label shapes in the contract name it.
      let name;
      if (labels.role === "chief-of-staff") name = "CoS";
      else if (!labels.role || labels.role === "chief-engineer") {
        if (typeof labels.task !== "string" || !labels.task ||
            typeof labels.title !== "string" || !labels.title) continue;
        name = `${labels.role === "chief-engineer" ? "CE " : ""}${labels.task} ${labels.title}`;
      } else continue;
      const tabKey = labels.role === "chief-of-staff" ? "cos" : labels.task;
      if (tab.id !== `${prefix}andon-${slug}-${tabKey}`) continue;
      rename("space", space, work ? `${slug} work` : slug);
      rename("tab", tab, name);
    }
    for (const [command, args] of commands.values()) run(command, args);
    return div();
  }
}
