# 下次编译一起改的三件事 ✅ 已完成

> 2026-10-26 已全部实现并提交到 commit `34eb9de`，**尚未推送至 GitHub main（不触发 CI）**。
> 下次 push main 后将自动触发云编译。

---

## 1) 网口顺序 ✅

CI 中已添加 DTS sed patch（`build.yml` → `Prepare custom packages & patches` step），编译时自动执行：
```bash
DTS=target/linux/qualcommax/files/arch/arm64/boot/dts/qcom/ipq6000-cmiot.dtsi
sed -i 's/label = "lan3"/label = "LAN_TMP"/; s/label = "lan1"/label = "lan3"/; s/label = "LAN_TMP"/label = "lan1"/' "$DTS"
```
刷机后验证：`lan1 ↔ lan3` 对应关系翻转，`brctl showmacs br-lan` 确认。

## 2) freellmapi ✅ → 已替换为自研 Go 轻量网关

freellmapi（Node 458MB）不可行，已改为 **自研 Go 网关**：
- 纯 Go 标准库，零依赖，交叉编译产物约 8–15 MB
- Go 源码自包含在 `ai-gateway/luci-app-ai-gateway/src/`
- `.config` 已添加 `CONFIG_PACKAGE_luci-app-ai-gateway=y`
- `build.yml` 将 `luci-app-ai-gateway` 复制到 `openwrt/package/`，`make defconfig` 自动发现
- LuCI 管理界面：`admin/services/ai_gateway`（Lua CBI 配置页 + RPC ACL）
- init.d 脚本已支持 UCI `enabled` 开关

## 3) 更改配置等待过长 ✅

CI 中已添加 LuCI patch（`build.yml` → `Prepare custom packages & patches` step）：
- `feeds/luci/modules/luci-base/root/etc/config/luci`：`option rollback 90` → `20`
- `feeds/luci/modules/luci-base/ucode/controller/admin/uci.uc`：`max(timeout, 90)` → `max(timeout, 15)`

---

## 其他同步完成的变更

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
# 路径（LEDE master，2026-10-28 切换后）：target/linux/qualcommax/dts/ipq6000-ax18.dts（label 映射相同：dp1=lan3 dp2=lan2 dp4=lan1 dp5=wan）
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

**已验证的根因（2026-10-27 设备实测）**：
- **仅改 `/etc/config/luci` 的 `rollback 90→0` 无效**：LibWrt 25.12 的 LuCI 是上游 openwrt-25.12 前端，`config internal apply` 是 ImmortalWrt 语法，前端 JS **不识别**，"保存并应用"仍走 `apply_rollback`（90 s 回滚）。
- **前端 `ui.js` 的 `initTabGroup` 崩溃**（tab 全空时 `selected` 为 null → `menu.childNodes[selected].classList` 报 TypeError）：中断页面初始化链，导致 **"保存并应用"按钮事件未绑定**（点了没反应）、部分页面卡"加载视图中…"。
- **老式 lua CBI**（含自研 ai-gateway）默认 `apply_on_parse=nil`：保存并应用走前端 XHR apply（依赖 JS + 90 s 回滚）；`apply_on_parse=true` 时后端 `uci:apply(true)` 立即 commit+apply。

**生效的修复（三处源码补丁，已设备实测通过，build.yml Step 4 已带上）**：
1. `feeds/luci/modules/luci-base/htdocs/.../resources/ui.js`（initTabGroup 兜底）：
   ```bash
   sed -i 's/selected=i;break;}}}/selected=i;break;}}}if(selected===null)selected=0;/' "$UIJS"
   ```
2. `feeds/luci/modules/luci-base/htdocs/.../resources/ui.js`（保存并应用恒走 apply_unchecked，立即生效无 90 s 回滚）：
   ```bash
   sed -i "s/apply_rollback':'apply_unchecked/apply_unchecked':'apply_unchecked/" "$UIJS"
   ```
3. `feeds/luci/modules/luci-base/luasrc/model/cbi.lua`（Map 默认 apply_on_parse=true，老 CBI 保存即后端立即 commit+apply）：
   ```bash
   sed -i 's/self.apply_on_parse = nil/self.apply_on_parse = true/' "$CBI"
   ```
   （编译期路径用 `find feeds/luci/modules/luci-base -name ui.js -path '*luci-static/resources*'` / `-path '*model/cbi.lua'` 定位，见 build.yml）

**实测结果**：系统页改主机名 → 保存并应用 → **uci 立即落盘、无 pending、无回滚、无 90 s 等待**；initTabGroup 崩溃消失（console 无错误）。

**取舍说明**：全局 apply_unchecked 后，改网络类配置失败时**不再自动回滚**（符合"立即生效、应用（不检查）"的需求方明确倾向）；如需保留回滚保护，仅对特定页面改回 `apply_rollback`。

## 4) nlbw 带宽监控页卡"加载视图中…" ✅（build.yml Step 5）

**根因**：`view/nlbw/display.js` 的 `loadOUI()` 请求 `raw.githubusercontent.com/.../oui.json`（MAC 厂商库），GitHub 被墙时请求挂起 → `load()` 的 `Promise.all` 永不 resolve → 整页转圈。
**修复**：oui.json 请求加 3s 超时（`Promise.race`），超时放弃不阻塞页面：
```bash
awk '{gsub(/request\.get\(url,\{cache:true\}\)/,"Promise.race([request.get(url,{cache:true}),new Promise(function(rs){setTimeout(function(){rs(null)},3000)})])");print}' "$NLBWDISP" > /tmp/n && mv /tmp/n "$NLBWDISP"
```
**实测**：nlbw 页 5s 内渲染、流量数据显示、无错误（oui 请求挂起也不再阻塞）。

## 5) KMS / SmartDNS 默认启用 ✅（build.yml Step 6）

设备实测：`vlmcsd.config.enabled=1` → 1688 监听（IPv4+IPv6）；`smartdns.@smartdns[0].enabled=1` → smartdns 接管 53、dnsmasq 保留 DHCP/本地域、LAN DNS 解析正常。build.yml Step 6 对包源码默认配置 sed `option enabled '0'→'1'`。

## 6) AI 网关：智谱 provider 已接入并端到端验证 ✅ / 前端 addremove 兼容问题 ⚠️

- 设备实测（2026-10-28）：双 provider（OpenRouter + 智谱AI）聚合成功，**fallback 机制验证通过**（`glm-4-flash` → OpenRouter 400 → 自动切智谱AI → ok:true，返回正确内容）。
- 智谱免费模型 `glm-4-flash` 可用（国内直连，无需翻墙，不受 OpenRouter 免费额度限制）。
- **⚠️ 前端 bug**：LuCI 25.12 前端下，老式 CBI（ai_gateway.lua）的"添加 provider"（addremove）**保存不生效**（新增 section 未写入 uci，页面显示正常但保存后丢失）。绕开方式：SSH `uci add ai-gateway provider` + 写 config.json。后续可选：把 ai_gateway.lua 迁移到新 CBI 或修 addremove 兼容。


