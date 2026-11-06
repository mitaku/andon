// How an agent is named, the same everywhere:
// a tab the person named, else the agent's own title when it says something,
// else the program's name. A terminal with no title of its own reports its address (t9) as one.

// Drops a leading spinner or mark ("✳ fix login").
const clean = (s) => (s ?? "").replace(/^[^\p{L}\p{N}]+/u, "").trim();

// A title says something unless it is a path, user@host, or the program's own name.
export function tellingTitle(title, program, address) {
  const t = clean(title);
  if (!t) return null;
  if (address && t === address.split("/").pop()) return null;
  if (/^[~/]|^[A-Za-z]:[\\/]/.test(t)) return null;
  if (/^[^\s@]+@[^\s@]+$/.test(t.split(/[\s:]/)[0])) return null;
  const own = [program?.name, program?.key].filter(Boolean).map((n) => n.toLowerCase());
  if (own.includes(t.toLowerCase())) return null;
  return t;
}

export function displayName(agent, tab) {
  if (tab?.named) return tab.name;
  return tellingTitle(agent.title, agent.program, agent.address) ?? agent.program?.name ?? agent.address;
}
