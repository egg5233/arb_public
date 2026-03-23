#!/bin/bash
set -e

# 推送到公開 repo（無歷史紀錄）
# 用法：bash push-public.sh

PUBLIC_REPO="https://github.com/egg5233/arb_public.git"
VERSION=$(cat VERSION 2>/dev/null || echo "unknown")

TMPDIR=$(mktemp -d)
git archive HEAD | tar -x -C "$TMPDIR"
rm -rf "$TMPDIR/.claude"

cd "$TMPDIR"
git init -b main
git remote add origin "$PUBLIC_REPO"
git add -A
git commit -m "update v${VERSION}"
git push -u origin main --force

cd -
rm -rf "$TMPDIR"

echo "已推送 v${VERSION} 到 ${PUBLIC_REPO}"
