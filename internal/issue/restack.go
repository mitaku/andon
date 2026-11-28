package issue

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"andon/internal/contract"
	"andon/internal/ledger"
)

// restack は andon issue restack: 前の課題が done になった積んだ課題の、自分の commit だけを既定の branch の先へ移す（ADR-0067 決めたこと 13）。
// 担当の端末で doing の課題に打つ。積んでいない課題と、前の課題がまだ done でない課題では何も変えずに 1 行出して 0 で終わる。
//
// 順は 1 つ: 移す先（既定の branch の先）を予約キー restack_onto に控える → git rebase --onto <移す先> <stacked_base> →
// PR が在り向き先が既定の branch でなければ付け替える（push より先。push が CI を既定の branch を base にして回す）→ push →
// draft を外す → stacked_on・stacked_base・restack_onto を消す。PR がまだ無ければ push とキーを消すことだけをする。
// 打ち直しでは済んだ rebase を打ち直さない（main の commit を積み直そうとして偽の衝突で止まる。事実 17）:
// restack_onto が在り、rebase の途中なら止まり、HEAD がその子孫なら続きから進み、どちらでもなければ控えを消して初めから。
func (c *ctx) restack(stdout io.Writer) error {
	if st := c.issue.Str("status"); st != "doing" {
		return fail("%s は %s（andon issue restack は doing の課題にだけ打てる。直しを受けたら kanban start で doing に戻してから）", c.id, st)
	}
	on, base, ok := ledger.Stacked(c.l.Log(), c.issue, c.l.ChiefOfStaff())
	if !ok {
		fmt.Fprintf(stdout, "%s は積んでいない（信じられる stacked_on・stacked_base が無い）。載せ替えるものは無い\n", c.id)
		return nil
	}
	p, err := c.l.Show(on)
	if err != nil {
		return fail("前の課題 %s が kanban で読めない: %v", on, err)
	}
	if st := p.Str("status"); st != "done" {
		fmt.Fprintf(stdout, "前の課題 %s がまだ done でない（%s）。載せ替えは前の課題が main に入ってから（何も変えていない）\n", on, st)
		return nil
	}
	dir, branch := c.issue.Path("worktree"), c.issue.Str("branch")
	if dir == "" || branch == "" {
		return fail("issue.yaml に worktree か branch が無い")
	}
	def := defaultBranch(dir)
	if def == "" {
		return fail("origin の既定の branch が分からない（git ls-remote --symref origin HEAD・git fetch が通らない）")
	}
	before, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		return fail("HEAD が読めない: %v", err)
	}
	onto := c.issue.Str("restack_onto")
	if onto != "" {
		switch {
		case rebasing(dir):
			return fail("載せ替えの rebase の途中。衝突を解いて git rebase --continue で済ませてから andon issue restack %s を打ち直す", c.id)
		case ancestor(dir, onto, "HEAD"):
			// rebase は済んでいる。続き（向き先・push・draft・キー）から進む
		default:
			// 担当が rebase をやめた: 控えを消して初めからやり直す
			if _, err := c.l.Run("", "set", c.id, "restack_onto="); err != nil {
				return fail("restack_onto を消せない: %v", err)
			}
			onto = ""
		}
	}
	if onto == "" {
		if onto, err = git(dir, "rev-parse", "refs/remotes/origin/"+def); err != nil || !contract.IsSHA(onto) {
			return fail("origin/%s が読めない: %v", def, err)
		}
		if _, err := c.l.Run("", "set", c.id, "restack_onto="+onto); err != nil {
			return fail("restack_onto を書けない: %v", err)
		}
		if _, err := git(dir, "rebase", "--onto", onto, base); err != nil {
			if rebasing(dir) {
				return fail("載せ替えの rebase が衝突した。衝突を解いて git rebase --continue で済ませてから andon issue restack %s を打ち直す（%v）", c.id, err)
			}
			return fail("載せ替えの rebase ができない（直してから andon issue restack %s を打ち直す）: %v", c.id, err)
		}
	}
	head, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		return fail("HEAD が読めない: %v", err)
	}
	rep, err := c.previous()
	if err != nil {
		return err
	}
	pr := ""
	if rep != nil {
		pr = rep.PR
	}
	if pr == "" {
		// 報告に pr がまだ無くても PR は出ていることがある（PR を出してから andon issue report --pr の前に打った）。
		// branch から引き、付け替えと draft を外すことを飛ばさない
		if pr, err = branchPR(c.issue.Str("repo"), branch); err != nil {
			return err
		}
	}
	draft := false
	if pr != "" {
		v, err := restackPR(pr)
		if err != nil {
			return err
		}
		switch v.State {
		case "OPEN":
		case "CLOSED":
			// GitHub が向き先を付け替えずに閉じた（前の課題の branch が消えた）: 開き直しを試みる
			if out, err := exec.Command("gh", "pr", "reopen", pr).CombinedOutput(); err != nil {
				return fail("PR %s が閉じられていて開き直せない（%s）。PR を出し直して andon issue report --pr で書き換え、andon issue restack %s を打ち直す", pr, firstLine(string(out)), c.id)
			}
		default:
			return fail("PR %s が %s。載せ替えない（報告に書いて Chief of Staff へ返す）", pr, v.State)
		}
		if v.Base != def {
			if out, err := exec.Command("gh", "pr", "edit", pr, "--base", def).CombinedOutput(); err != nil {
				return fail("PR %s の向き先を %s に付け替えられない（打ち直す）: %s", pr, def, firstLine(string(out)))
			}
		}
		draft = v.Draft
	}
	// origin の branch が手元と違っていても（GitHub の側で書き直された）、手元から載せ替えたもので上書きする
	remote, err := git(dir, "ls-remote", "--heads", "origin", "refs/heads/"+branch)
	if err != nil {
		return fail("origin の branch %s を確かめられない（打ち直す）: %v", branch, err)
	}
	if f := strings.Fields(remote); len(f) > 0 {
		if _, err := git(dir, "push", "--force-with-lease=refs/heads/"+branch+":"+f[0], "origin", "HEAD:refs/heads/"+branch); err != nil {
			return fail("載せ替えた branch を push できない（andon issue restack %s を打ち直す。rebase はやり直さない）: %v", c.id, err)
		}
	} else if _, err := git(dir, "push", "-u", "origin", "HEAD:refs/heads/"+branch); err != nil {
		return fail("載せ替えた branch を push できない（andon issue restack %s を打ち直す。rebase はやり直さない）: %v", c.id, err)
	}
	if draft {
		if out, err := exec.Command("gh", "pr", "ready", pr).CombinedOutput(); err != nil {
			return fail("PR %s の draft を外せない（andon issue restack %s を打ち直す）: %s", pr, c.id, firstLine(string(out)))
		}
	}
	if _, err := c.l.Run("", "set", c.id, "stacked_on=", "stacked_base=", "restack_onto="); err != nil {
		return fail("stacked_on・stacked_base・restack_onto を消せない（andon issue restack %s を打ち直す）: %v", c.id, err)
	}
	fmt.Fprintf(stdout, "%s を %s の先（%.12s）へ載せ替えた（%.12s → %.12s）。前の課題 %s の上に積んでいない、ふつうの課題になった。検査を通し、andon issue report で書き直し、レビューが引き継がれなければ andon issue review-request、引き継がれれば andon issue submit へ進む\n",
		c.id, def, onto, before, head, on)
	return nil
}

type restackView struct {
	State string `json:"state"`
	Base  string `json:"baseRefName"`
	Draft bool   `json:"isDraft"`
}

func restackPR(pr string) (restackView, error) {
	cmd := exec.Command("gh", "pr", "view", pr, "--json", "state,baseRefName,isDraft")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	var v restackView
	if err != nil || json.Unmarshal(out, &v) != nil || v.State == "" {
		return v, fail("PR %s が読めない（打ち直す）: %v %s", pr, err, firstLine(errb.String()))
	}
	return v, nil
}

// branchPR は、課題の branch から出ている PR の URL を GitHub に聞く（無ければ空）。repo は issue.yaml の repo（github.com/<owner>/<name>）。
func branchPR(repo, branch string) (string, error) {
	slug := strings.TrimPrefix(repo, "github.com/")
	if slug == repo || slug == "" {
		return "", fail("issue.yaml の repo が github.com/<owner>/<name> でない（%q）。PR を branch から引けない", repo)
	}
	cmd := exec.Command("gh", "pr", "list", "--repo", slug, "--head", branch, "--state", "all", "--json", "url", "--jq", ".[0].url // \"\"")
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		return "", fail("branch %s の PR を GitHub に聞けない（打ち直す）: %v %s", branch, err, firstLine(errb.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// rebasing は worktree dir で rebase が途中か（git の rebase-merge・rebase-apply の置き場が在る）。
func rebasing(dir string) bool {
	for _, p := range []string{"rebase-merge", "rebase-apply"} {
		path, err := git(dir, "rev-parse", "--git-path", p)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			path = dir + "/" + path
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// ancestor は worktree dir で a が b の祖先か（同じ commit も含む）。
func ancestor(dir, a, b string) bool {
	_, err := git(dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}
