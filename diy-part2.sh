#!/bin/bash
# ============================================================
# diy-part2.sh - 在 feeds install 之后、make defconfig 之前执行
# 执行位置：openwrt 源码根目录（当前工作目录即为 openwrt/）
# 用途：修改源码/默认配置、删除默认包、打补丁
# ============================================================

# ---- 示例 1：修改默认 LAN IP（取消注释启用）----
# sed -i 's/192.168.1.1/192.168.6.1/g' package/base-files/files/bin/config_generate

# ---- 示例 2：删除会导致编译报错的默认包（如 vlmcsd，取消注释启用）----
# sed -i 's/^CONFIG_PACKAGE_luci-app-vlmcsd=y/# CONFIG_PACKAGE_luci-app-vlmcsd is not set/' .config

# ---- 示例 3：设置默认主题为 argon（取消注释启用）----
# sed -i 's/luci-theme-bootstrap/luci-theme-argon/g' feeds/luci/collections/*/Makefile

# ---- 示例 4：给 cmiot_ax18 设备定义追加板卡数据包（仅供进阶调试，见 README §4）----
# 需先自行准备 board-cmiot_ax18.ipq6018 校准文件并放入 ipq-wifi 包；
# 以下演示如何给设备定义追加 DEVICE_PACKAGES（用 awk 在 DEVICE_DTS_CONFIG 行后插入）：
# awk '/define Device\/cmiot_ax18/{f=1} f && /DEVICE_DTS_CONFIG/{print; print "\tDEVICE_PACKAGES := ipq-wifi-cmiot_ax18"; next} {print}' \
#   target/linux/qualcommax/image/ipq60xx.mk > /tmp/ipq60xx.mk && mv /tmp/ipq60xx.mk target/linux/qualcommax/image/ipq60xx.mk

# ---- LuCI 修复：保存并应用立即生效，避免 90s 回滚确认报错 ----
# 现象：LEDE 25.12 的 LuCI 中"保存并应用"走 apply_rollback（90s 确认窗口），
#       断流/超时时会触发回滚并报错。修复：前端所有 apply 恒走 apply_unchecked，
#       UCI 立即 commit+apply，无回滚确认窗口。
# 新版 ui.js 为三目运算写法，旧版为对象简写，两种模式都处理；未命中仅告警不中断。
UIJS=$(find feeds/luci/modules/luci-base -name ui.js -path '*luci-static/resources*' | head -1)
echo "UIJS=$UIJS"
if [ -n "$UIJS" ]; then
  if grep -q "checked ? 'apply_rollback' : 'apply_unchecked'" "$UIJS"; then
    sed -i "s/checked ? 'apply_rollback' : 'apply_unchecked'/'apply_unchecked'/g" "$UIJS"
    echo "PATCH apply-unchecked APPLIED (new-style ternary)"
  elif grep -q "apply_rollback':'apply_unchecked" "$UIJS"; then
    sed -i "s/apply_rollback':'apply_unchecked/apply_unchecked':'apply_unchecked/" "$UIJS"
    echo "PATCH apply-unchecked APPLIED (legacy object)"
  else
    echo "WARN: ui.js apply_rollback/apply_unchecked pattern not found"
  fi
  grep -n "apply_unchecked" "$UIJS" | head -5
else
  echo "WARN: ui.js not found, skip apply-unchecked patch"
fi

echo "diy-part2.sh 执行完成（本次未启用额外操作，按需编辑本文件）"
