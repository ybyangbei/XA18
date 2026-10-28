#!/bin/bash
# ============================================================
# diy-part1.sh - 在 feeds update 之前执行（仓库根目录）
# 用途：添加/修改 feeds 源、准备自定义内容
# 说明：此时 openwrt 源码已克隆完成，但 feeds 尚未更新。
# ============================================================

# ---- 示例：追加自定义 feed（如 openwrt-passwall，取消注释启用）----
# cat >> openwrt/feeds.conf.default <<EOF
# src-git passwall_packages https://github.com/xiaorouji/openwrt-passwall-packages.git
# src-git passwall https://github.com/xiaorouji/openwrt-passwall.git
# EOF

# ---- 示例：移除默认 feed（如不需要 helloworld，取消注释启用）----
# sed -i '/helloworld/d' openwrt/feeds.conf.default

echo "diy-part1.sh 执行完成（本次未启用额外操作，按需编辑本文件）"
