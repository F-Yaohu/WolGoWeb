# WolGoWeb Docker 部署指南

> 本项目基于 [xiaoxinpro/WolGoWeb](https://github.com/xiaoxinpro/WolGoWeb) 进行了功能增强和界面优化。

## 项目说明

WolGoWeb 是一款远程唤醒 (Wake-on-LAN) Web 管理工具，用于在局域网内通过 Web 界面管理和唤醒设备。

**在使用前请确认：**
- 目标设备的主板支持 WOL 功能
- 已在 BIOS 中启用 WOL
- 网卡驱动已配置 WOL 选项

## 主要改进

基于原版 WolGoWeb，本版本增加了以下功能：

### 🎨 界面优化
- 全新的现代化 UI 设计（基于 Vue 3 + Pico.css）
- 响应式布局，完美适配 PC 和移动端
- 设备在线状态可视化（绿色/灰色指示灯）
- 设备列表排序和筛选功能
- Toast 通知取代弹窗，交互更流畅

### ⚡ 功能增强
- **设备管理**：添加、删除、编辑设备信息
- **实时状态**：自动检测设备在线状态
- **局域网扫描**：自动发现网络中的设备
- **智能过滤**：自动过滤多播地址和无效 MAC
- **数据持久化**：设备信息自动保存到 `devices.json`
- **登录认证**：可选的用户名密码保护

### 🔧 技术改进
- 优化的 Ping 扫描算法（支持并发和重试）
- 线程安全的设备状态更新
- 智能路径检测（兼容 Docker 和本地环境）
- MAC 地址自动格式化和验证

## Docker Compose 部署（推荐）

创建 `docker-compose.yml` 文件：

```yaml
version: '3'
services:
  wol-go-web:
    image: zy1234567/wol-go-web:latest
    container_name: WolGoWeb
    restart: unless-stopped
    network_mode: host
    environment:
      - PORT=9090          # Web 服务端口
      - KEY=false          # API 密钥，false 则关闭
      - USERNAME=admin     # 登录用户名（留空则无需登录）
      - PASSWORD=admin     # 登录密码
    volumes:
      - ./data:/web/data   # 数据持久化目录
```

启动容器：

```bash
docker-compose up -d
```

访问 `http://服务器IP:9090` 即可使用。

## Docker 命令部署

快速启动（使用默认配置）：

```bash
docker run -d --net=host zy1234567/wol-go-web-ui:latest
```

自定义配置：

```bash
docker run -d --net=host \
  -e PORT=9090 \
  -e USERNAME=admin \
  -e PASSWORD=admin \
  -v ./data:/web/data \
  zy1234567/wol-go-web:latest
```

## 环境变量说明

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `PORT` | Web 服务端口 | `9090` |
| `USERNAME` | 登录用户名（留空则不需要登录） | `admin` |
| `PASSWORD` | 登录密码 | `admin` |
| `KEY` | API 密钥验证（false 则关闭） | `false` |
| `WEB` | 是否启用 Web 界面 | `true` |

## 数据持久化

容器使用 `/web/data` 目录存储设备数据：

```bash
./data/devices.json  # 设备列表数据
```

挂载此目录可确保容器重启或升级后数据不丢失。

## 端口说明

由于 Wake-on-LAN 需要发送广播包到局域网，**必须使用 `--net=host` 模式**，否则无法正常唤醒设备。

## 常见问题

**Q: 为什么要用 host 网络模式？**  
A: WOL 协议需要发送网络广播包，bridge 模式下无法访问宿主机网络接口。

**Q: 扫描不到设备怎么办？**  
A: 确保设备已开机且在同一局域网内，防火墙允许 ICMP ping。

**Q: 唤醒失败怎么办？**  
A: 检查目标设备的 BIOS 设置、网卡 WOL 选项和网线连接。

## 原项目链接

- 原作者：[xiaoxinpro/WolGoWeb](https://github.com/xiaoxinpro/WolGoWeb)
- 本项目：增强版带现代化 UI

## 许可证

本项目继承原项目的开源协议。

[https://github.com/xiaoxinpro/WolGoWeb](https://github.com/xiaoxinpro/WolGoWeb)

