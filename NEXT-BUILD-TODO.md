# 下次编译一起改的三件事（2026-10-26 记录）

设备现状：LibWrt 25.12-SNAPSHOT r0-3a3d0b0 / kernel 6.12.108 / CMIOT AX18
登录：`root@192.168.2.1`，密码 `OOoo0000`（LuCI 只有 HTTP 192.168.2.1:80）
固件产物：`Desktop/AX18固件/`（sysupgrade.bin / factory.ubi / manifest / sha256sums）

---

## 1) 网口顺序不对（lan1 / lan3 对调）

实际映射（来自设备树 `ipq6000-cmiot.dtsi` + 现场验证）：

| 内核名 | DT 节点 | PHY | 说明 |
|---|---|---|---|
| lan1 | &dp4 | qca8075_3 | 用户电脑所在口（brctl port 1） |
| lan2 | &dp2 | qca8075_1 | |
| lan3 | &dp1 | qca8075_0 | |
| wan  | &dp5 | qca8075_4 | |

判断：软件命名与机壳丝印相反，典型是 **lan1 ↔ lan3 互换**。

**待确认**：把网线插在机壳印着 `LAN1` 的口上，看系统里是 `lan1` 还是 `lan3`
（或直接确认三个 LAN 口从左到右的丝印顺序）。

修法 A（推荐，随固件改）：CI 里克隆完源码后对设备树 label 做一次互换

```bash
# 路径（LibWrt 25.12-nss）：target/linux/qualcommax/files/arch/arm64/boot/dts/qcom/ipq6000-cmiot.dtsi
DTS=target/linux/qualcommax/files/arch/arm64/boot/dts/qcom/ipq6000-cmiot.dtsi
sed -i 's/label = "lan3"/label = "LAN_SWAP"/; s/label = "lan1"/label = "lan3"/; s/label = "LAN_SWAP"/label = "lan1"/' "$DTS"
grep -n 'label = "' "$DTS"   # 校验：dp1=lan1, dp2=lan2, dp4=lan3, dp5=wan
```

修法 B（不重刷，临时验证）：早期启动脚本（netifd 之前）重命名 netdev

```sh
# /etc/preinit.d/99-port-rename（示例）
ip link set lan1 down; ip link set lan1 name lan_tmp
ip link set lan3 name lan1; ip link set lan_tmp name lan3
```

> 注意：`luci-mod-status` 的端口图来自 `/etc/board.json`（ports: lan1 lan2 lan3），换名后仍有效。
> 改完后 `/etc/config/network` 的 `ports` 不用动。

---

## 2) 把 freellmapi 编译进固件（技术上不可行，需换方案）

源码：`C:\Users\杨培\Doubao\chats\2026-10-25\new-chat\freellmapi`
（Node/TS monorepo：server / client / desktop / cli）

阻塞点：
- server 依赖 `better-sqlite3`（原生模块，需交叉编译）、`sharp`、express；`node_modules` 共 **458 MB**
- Dockerfile 需要 **Node 24**
- AX18 **没有 USB 口**（`/sys/bus/usb` 无设备、无 `/dev/sd*`），无法外挂存储
- overlay 总共 **52.4 MB**，当前可用 **49 MB**；整包固件 33 MB

结论：Node 运行时（50 MB+）都放不下，硬塞会导致无法升级/空间爆掉。

可选方案（待用户拍板）：
1. **留在电脑上跑**（现状）——路由器只做 LAN 访问 + Tailscale/DDNS，家里设备直连 `电脑IP:3001`；
2. 放到常开设备（NAS / Docker 小主机）跑项目自带 `docker-compose.yml`；
3. 在路由器上实现**极简版**（ucode 或 Go，只做 `/v1/chat/completions` 转发与 key 轮换），与上游项目不同代码。

相关线索（若做方案 3）：
- 上游 server 的 DB 层在 `server/src/db/index.ts`，可用 Node 内置 `node:sqlite`（Node ≥22.13）替代 better-sqlite3，去掉原生依赖
- 目录 `server/src` 有 653 个 .ts 文件、7.6 MB —— 说明逻辑较重，极简版只能取核心转发

---

## 3) 更改配置等待时间过长（LuCI 90 秒回滚窗口）

实测（真实 LuCI 接口）：
| 操作 | 耗时 |
|---|---|
| `admin/uci/apply_rollback`（DHCP 改动） | 0.11 s |
| `admin/uci/apply_rollback`（network 改动） | 0.13 s |
| `admin/uci/confirm` | 0.10 s |
| `/sbin/reload_config` | 0.09 s |
| LuCI 页面 | 0.06–0.11 s |

服务端不慢；慢的是 **LuCI 的失败回滚确认窗口 = 90 秒**：
`/etc/config/luci` → `config internal apply { rollback 90; holdoff 4; timeout 5; display 1.5 }`

触发条件（会转圈数到 90 s 然后提示"配置已回滚"）：
- 改网络类设置后浏览器短暂连不上路由器；页面会话过期（实测旧会话 apply 直接 403）；手机切后台轮询中断。

修改点（两处必须一致）：

1. `feeds/luci/modules/luci-base/root/etc/config/luci`
   `option rollback 90` → `option rollback 20`
   ```bash
   sed -i 's/^\(\s*option rollback\s*\)90$/\120/' feeds/luci/modules/luci-base/root/etc/config/luci
   ```
2. `feeds/luci/modules/luci-base/ucode/controller/admin/uci.uc`（约第 52 行）
   `timeout: max(timeout, 90)` → `timeout: max(timeout, 10)`
   ```bash
   sed -i 's/max(timeout, 90)/max(timeout, 10)/' feeds/luci/modules/luci-base/ucode/controller/admin/uci.uc
   ```

临时规避（不重刷）：网络类改动点 **"应用（不检查）"**（Apply unchecked），完全不进回滚等待。

---

## 备注（不要误改）

- 本固件 **故意不含 WiFi**（用户要求）：`.config` 里 `kmod-ath11k*`、`ath11k-firmware-*`、
  `ipq-wifi-cmiot_ax18`、`wpad-*` 全是 not set。后续重编不要开回来。
- `luci.apply.*` 的其它值（holdoff/timeout/display）保持原样，只改 rollback。
- 现场验证过的运行状态（供对比基线）：RAM 929 MB、overlay 52.4 M/剩 49 M、overlay 已用 688 K、
  WAN PPPoE 100.70.79.26 (CGNAT)、IPv6 /60 PD 正常、NSS/ECM 卸载生效（success 11829 / fail 0）、
  CPU 温度 40–41.5 ℃、测速约 313 Mbps。
