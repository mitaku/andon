# tests/*.sh が source する: 文の中の andon のコマンドを、andon の葉の表と照らす（ADR-0053 決めたこと 21）。
#   andon_leaves                     葉の名前を1行ずつ出す（andon help --all の行「  <葉>  <打つ者>  <説明>」から読む。$andon が要る）
#   andon_refs md|code <ファイル…>   andon <名詞> [<動詞>] を「<ファイル>:<行>\t<名詞>\t<動詞>」で出す。
#                                    md は `` で囲んだ語とコードの囲み（```）の中の行だけ、code は全部の行を見る。
#                                    行に <!-- andon の古い名前 --> があれば、その行は見ない（用語集が古い名前を挙げる行）
#   andon_bad_refs <葉の一覧のファイル>  標準入力の andon_refs の行のうち、表に無いものを出す
# 名詞と動詞は英小文字とハイフンの語だけを見る（<id>・$sub・… は名詞にも動詞にも数えない）。
# 照らし方は andon の振り分けと同じ: 2語目が動詞の形で、名詞に2語の葉があれば「名詞 動詞」が表に要る。
# 名詞が1語の葉なら、2語目はその葉の引数（andon open sample-site・andon gate S-0012）。名詞だけは、2語の葉を持つ名詞なら通る。

andon_leaves() {
  "$andon" help --all | sed -n 's/^  \([a-z][a-z-]*\( [a-z][a-z-]*\)\{0,1\}\)  .*/\1/p'
}

andon_refs() {
  local mode=$1
  shift
  perl -CSDA -Mutf8 -e '
    my $mode = shift;
    my ($fence, $file) = (0, "");
    while (my $line = <>) {
      if ($ARGV ne $file) { ($fence, $file) = (0, $ARGV) }
      if ($line !~ /<!-- andon の古い名前 -->/) {
        if ($mode eq "md" && $line =~ /^\s*```/) {
          $fence = !$fence;
        } else {
          my @spans = ($mode eq "code" || $fence) ? ($line) : ($line =~ /`([^`]+)`/g);
          for my $t (@spans) {
            $t =~ s/[`\$(){};|&"\x27=\[\]「」（）、。・／，：:！？]/ /g;
            my @w = split " ", $t;
            for my $i (0 .. $#w) {
              next unless $w[$i] eq "andon";
              my $n = $w[$i + 1] // "";
              my $v = $w[$i + 2] // "";
              next unless $n =~ /^[a-z][a-z-]*$/;
              $v = "" unless $v =~ /^[a-z][a-z-]*$/;
              print "$ARGV:$.\t$n\t$v\n";
            }
          }
        }
      }
      close ARGV if eof;
    }' "$mode" "$@"
}

andon_bad_refs() {
  perl -CSDA -e '
    open my $f, "<", shift or die "葉の一覧が読めない\n";
    my (%leaf, %kids);
    while (<$f>) {
      chomp;
      next if $_ eq "";
      $leaf{$_} = 1;
      my ($n, $v) = split / /;
      $kids{$n} = 1 if defined $v;
    }
    die "葉の一覧が空\n" unless %leaf;
    while (<STDIN>) {
      chomp;
      my ($at, $n, $v) = split /\t/, $_, -1;
      my $ok = ($v ne "" && $kids{$n}) ? $leaf{"$n $v"} : ($leaf{$n} || $kids{$n});
      print "$at: andon $n" . ($v ne "" ? " $v" : "") . "\n" unless $ok;
    }' "$1"
}
