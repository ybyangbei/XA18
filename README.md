# CMIOT AX18（和目 AX18）OpenWrt 云编译工程

基于 **coolsnowwolf/lede** 源码 + **GitHub Actions** 的云编译工程，一键产出 CMIOT AX18（IPQ6000）固件。已针对 lede 当前源码（2026-10 实测镜像）核实全部关键配置。

---

## ⚠️ 先读：两点关键结论

1. **原版 lede 编译的 AX18 固件 WiFi 不可用**。
   - lede 的设备树中 AX18 的 WiFi 节点为 `status = "disabled"`（`target/linux/qualcommax/dts/ipq6000-ax18.dts`）；
   - lede 的 `ipq-wifi` 包（`package/firmware/ipq-wifi`）不含 `cmiot_ax18` 的板卡校准数据（ALLWIFIBOARDS 无此项）；
   - 因此原版 lede 固件只能作**有线路由 / 旁路由**使用（CPU 性能足够跑代理等）。
   - **要 WiFi**：把工作流里 `REPO_URL` 一行改为 `https://github.com/AnClark/lede-zn-m2-wifi`（同为 lede 生态的 WiFi 增强 fork，专为 ZN-M2 / CMIOT AX18 制作），其余不用动。详见 [§4](#4-wifi-支持说明)。

2. **lede 平台已从 `ipq60xx` 独立目标并入 `qualcommax`**。当前设备配置符号为：
   `CONFIG_TARGET_qualcommax=y` / `CONFIG_TARGET_qualcommax_ipq60xx=y` / `CONFIG_TARGET_qualcommax_ipq60xx_DEVICE_cmiot_ax18=y`（本项目 `.config` 已写好）。网上 2024 年前的老教程里的 `ipq60xx-generic-cmiot_ax18` 命名已过时，勿照抄。

---

## 1. 文件清单

| 文件 | 作用 |
| --- | --- |
| `.github/workflows/build-openwrt.yml` | GitHub Actions 工作流：克隆 lede → 更新 feeds → 载入配置 → 编译 → 上传固件/发布 Release |
| `.config` | 种子配置：目标平台 cmiot_ax18 + LuCI + 常用组件（`make defconfig` 自动补全） |
| `diy-part1.sh` | feeds 更新前执行：添加自定义 feed 等 |
| `diy-part2.sh` | feeds 安装后执行：改默认 IP、删默认包、打补丁等 |
| `README.md` | 本说明 |

## 2. 快速开始（5 步）

1. **新建 GitHub 仓库**（私有即可），把本目录文件全部上传（保持目录结构 `.github/workflows/`）。
2. 进入仓库 **Actions** 页面 → 左侧选择 **Build OpenWrt - CMIOT AX18** → **Run workflow** → 确认。
3. 等待编译（通常 1.5~3 小时；可在该任务页面实时看日志）。
4. 完成后有两个下载入口：
   - **Actions 任务页底部**的 `cmiot-ax18-openwrt` 附件（保留 30 天）；
   - 仓库 **Releases** 页面（自动发布，无有效期）。
5. 解压后得到固件文件（见 §3）。

> 网络前提：push 代码、运行 Actions 都需要能正常访问 GitHub。国内网络通常需要代理，请自行准备。

## 3. 产物与刷机

### 3.1 产物文件

编译完成后在 `bin/targets/qualcommax/ipq60xx/` 下生成（随版本号略有差异）：

| 文件 | 用途 |
| --- | --- |
| `openwrt-qualcommax-ipq60xx-cmiot_ax18-squashfs-nand-factory.bin` | **过渡包**：用于原厂固件 → OpenWrt 的首次刷机 |
| `openwrt-qualcommax-ipq60xx-cmiot_ax18-squashfs-nand-sysupgrade.bin` | 升级包：用于 OpenWrt 内升级（保留配置） |
| `sha256sums` | 固件校验值，刷机前建议核对 |

### 3.2 首次刷机（原厂 → OpenWrt）

推荐"原厂后台直刷过渡包"路线（最稳，无需拆机）：

1. 电脑连 AX18，浏览器进原厂后台（默认 `192.168.1.1`，账号密码见机身标签）。
2. 进入 **高级设置 → 升级固件**，**取消勾选"保留配置"**。
3. 选择 `...-factory.bin` 过渡包 → 上传 → 升级。
4. 等待重启（约 5~10 分钟）进入 OpenWrt（默认 `192.168.1.1`，`root`，首次登录无密码，建议立即设置）。

> 进阶（可选）：刷不死 U-Boot 后可在 Web 救砖、随意刷机。需要 `ax18-mibib.bin`（分区扩容）、`uboot-cmiot-ax18-mod.bin` 等文件，涉及 mtd 写入，务必先备份原厂固件并确认 uboot 与内存（256M/1G）匹配，参考 §8 资料。

### 3.3 之后升级

OpenWrt 内 **系统 → 备份/升级 → 刷写新的固件**，选择 `...-sysupgrade.bin`（可保留配置）。

## 4. WiFi 支持说明

| 方案 | REPO_URL 设置 | WiFi | 说明 |
| --- | --- | --- | --- |
| **A（本工程默认）** | `https://github.com/coolsnowwolf/lede` | ❌ 不可用 | 原版 lede，设备树禁用 WiFi；可作有线路由/旁路由 |
| **B（推荐，要 WiFi）** | `https://github.com/AnClark/lede-zn-m2-wifi` | ✅ 可用 | lede 的 WiFi 增强 fork，专为 ZN-M2 / CMIOT AX18 维护（原 `lede-zn-m2` 已改名） |
| C（进阶） | `https://github.com/coolsnowwolf/lede` | 需自备板卡数据 | 自行准备 `board-cmiot_ax18.ipq6018` 校准文件并给 `ipq60xx.mk` 追加 `DEVICE_PACKAGES := ipq-wifi-cmiot_ax18`，见 `diy-part2.sh` 示例 4 |

切换方案只需改 `.github/workflows/build-openwrt.yml` 顶部的 `REPO_URL` 一行，`.config` 无需改动（fork 与 lede 使用同一设备名 `cmiot_ax18`；若 fork 内设备名有出入，以其 `target/linux/qualcommax/image/ipq60xx.mk` 中的 `TARGET_DEVICES` 为准）。

## 5. 自定义

### 5.1 增减软件包（.config）

- 直接编辑 `.config`，把需要的条目取消注释/改为 `=y`，不需要的删掉或改为 `# ... is not set`。
- `.config` 底部"可选插件"区已给出常见条目（SSR-Plus、passwall、zerotier、smartdns、samba4 等）。
- 注意：AX18 内存仅 **256MB**，插件别堆太多，否则运行期易 OOM。

### 5.2 添加第三方 feed（diy-part1.sh）

lede 已默认自带 `helloworld` feed（科学上网 SSR-Plus 等，直接勾选即可）。需要 passwall 等其他 feed 时，在 `diy-part1.sh` 取消注释示例并填上对应 git 地址。

### 5.3 系统默认设置（diy-part2.sh）

改默认 LAN IP、默认主题、删除默认包等，见文件内示例。

### 5.4 自定义 /etc 文件

在仓库根目录建 `files/` 目录，按 `files/etc/...` 的结构放入文件，编译时会覆盖到固件对应路径（工作流已支持）。

## 6. 常见问题

| 问题 | 处理 |
| --- | --- |
| 编译中途 OOM / 进程被杀 | 把工作流中 `make -j$(nproc)` 改为 `make -j2`（GitHub 免费 runner 4 核 16G，个别包会吃内存） |
| `make download` 失败 | 工作流已内置 3 次重试；仍失败多 Run 一次即可（下载有缓存） |
| 编译报某包错误 | 多数是该包与当前内核/依赖冲突：先在 `.config` 里去掉该包，或用 `diy-part2.sh` 示例 2 的方式删除 |
| 刷完没有 WiFi | 确认是否用的是原版 lede（方案 A 无 WiFi），换方案 B 源码重编 |
| Release 未生成 | 工作流默认发布到 Releases；如仓库禁用或 token 权限受限，可把 `UPLOAD_RELEASE` 改为 `"false"`，仅用 Actions 附件 |
| 想实时看编译过程 | 工作流暂未内置 SSH 调试；需要时可自行添加（搜索 "Actions SSH debug"） |

## 7. 硬件参考（已核对 DTS）

| 项目 | 参数 |
| --- | --- |
| 型号 | CMIOT AX18（和目 AX18；与兆能 ZN-M2 同主板方案） |
| SoC | 高通 IPQ6000，四核 Cortex-A53 @1.2GHz + 单核 1.5GHz NPU |
| 内存 | 256MB DDR（硬改 1G 需对应 uboot/cdt 文件） |
| 闪存 | 128MB NAND |
| 网络 | 3×LAN + 1×WAN 千兆（QCA8075 PHY） |
| WiFi | IPQ6000 集成 2.4G/5G WiFi6（原版 lede 中默认禁用，见 §4） |
| USB | 无 |

## 8. 参考资料

- coolsnowwolf/lede 仓库：<https://github.com/coolsnowwolf/lede>
- lede WiFi fork（ZN-M2 / CMIOT AX18）：<https://github.com/AnClark/lede-zn-m2-wifi>
- 云编译模板参考：<https://github.com/P3TERX/Actions-OpenWrt>
- 编译带 WiFi 的 ZN-M2/AX18 固件（少数派）：<https://sspai.com/>
- 刷机指北（ZN-M2 / AX18 快速入门，含 uboot/分区说明）：<https://evan618.github.io/evan.github.io/post/ZN-M2%EF%BC%8CAX18%20-kuai-su-ru-men-%20chao-xiang-xi-%20shua-ji-%20OpenWrt%20-zhi-bei-%20%26%20-chang-jian-wen-ti-hui-zong.html>
- OpenWrt 官方论坛（ZN-M2 / CMIOT AX18 支持讨论）：<https://forum.openwrt.org/t/openwrt-support-for-zn-m2-cmiot-ax18/247964>

---

*本工程依据 lede 源码 2026-10-27 镜像核实；lede 迭代较快，若日后平台/设备定义变化，以仓库 `target/linux/qualcommax/image/ipq60xx.mk` 为准。*
