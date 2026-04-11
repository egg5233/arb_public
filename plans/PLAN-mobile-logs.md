# Plan: Fix Mobile Log Display

## Problem
手機版日誌頁面文字垂直排列，因為 timestamp/level/module 都是 `shrink-0` 佔滿窄螢幕寬度，message 被壓到幾乎 0 寬度。

## Fix
`web/src/pages/Logs.tsx` line 119, 125:
1. 日誌容器加 `overflow-x-auto`（已有 `overflow-y-auto`）
2. 每行加 `whitespace-nowrap` + 移除 message 的 `break-all`
3. 像終端機一樣水平捲動看完整 log

## Changes
- Line 119: `overflow-y-auto` → `overflow-y-auto overflow-x-auto`
- Line 125: `flex gap-2` → `flex gap-2 whitespace-nowrap`
- Line 139: `break-all` → 移除（整行不換行，靠水平捲動）
