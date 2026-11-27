// 一覧の面の画面（ADR-0064 決めたこと 18〜25）。画面のファイルにデータを入れず、データは一覧の面の API から読む。
//   /         未読の報告（開いたときの絞り込み）
//   /all      30日の報告
//   /old      古い報告（最後に上がってから 30 日を過ぎたもの）
//   /r/<id>   報告。細い帯と、別オリジンの報告の面を出す枠
//   /cells    セルと鍵
import "./global.css";
import { createRouter } from "@solidjs/router";
import { render } from "@solidjs/web";
import { lazy } from "solid-js";
import * as stylex from "@stylexjs/stylex";
import { ReportList } from "./pages/list";
import { ReportFrame } from "./pages/report";
import { base } from "./ui";

// 色と字の決まりは html に付ける（画面の外の余白とスクロールの端も同じ色にする）
document.documentElement.className = stylex.props(base.root).className ?? "";

// セルの画面は開くことが少なく、ダイアログの部品が重いので、開いたときに読む
const Cells = lazy(() => import("./pages/cells").then((m) => ({ default: m.Cells })));

const Router = createRouter({
  routes: [
    { path: "/", component: () => <ReportList view="unread" /> },
    { path: "/all", component: () => <ReportList view="recent" /> },
    { path: "/old", component: () => <ReportList view="old" /> },
    { path: "/r/:id", component: ReportFrame },
    { path: "/cells", component: Cells },
    { path: "*404", component: () => <ReportList view="unread" /> },
  ],
});

render(() => <Router>{(props) => props.children}</Router>, document.getElementById("app") as HTMLElement);
