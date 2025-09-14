# LiuProxy Remote Server

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=for-the-badge&logo=go)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=for-the-badge&logo=docker)

`liuproxy-remote` 是 `LiuProxy v2.0` 项目的服务端组件。它是一个高性能、轻量级的Go应用程序，专门用于接收来自`liuproxy-local`或`liuproxy-gateway`客户端的加密隧道连接，并将流量安全地转发到目标互联网地址。

本项目已被从主项目 [`liuproxy-gateway`](https://github.com/your-username/liuproxy-gateway) 中拆分出来，以便于独立开发、部署和维护。

---

## 功能特性

*   **自研协议**: 完整支持`liuproxy v2.2`隧道协议，该协议基于WebSocket并使用AEAD加密（XChaCha20-Poly1305）。
*   **多路复用**: 在单一的WebSocket连接上高效地处理多个并发的TCP流和UDP会话。
*   **TCP & UDP 代理**: 提供完整的TCP和UDP代理能力。
*   **PaaS平台兼容**: 能够自动识别并使用PaaS平台（如Railway）注入的`PORT`环境变量，实现无缝部署。
*   **轻量高效**: 基于Go语言构建，资源占用小，性能卓越。
*   **Docker化**: 提供官方的、经过优化的多阶段构建`Dockerfile`，便于快速部署。

---

## 快速开始

推荐使用Docker进行部署。

### 1. 使用 `docker-compose` (推荐)

在您的服务器上创建一个`docker-compose.yml`文件：

```yaml
version: '3.8'

services:
  liuproxy-remote:
    image: your-dockerhub-username/liuproxy-remote:latest # 将其替换为您的Docker Hub镜像
    container_name: liuproxy_remote_server
    restart: unless-stopped
    ports:
      # 将服务器的 10089 端口映射到容器的 10089 端口
      - "10089:10089"
    volumes:
      # 将本地配置文件挂载到容器中，实现持久化配置
      - ./remote.ini:/app/configs/remote.ini
```

创建一个`remote.ini`配置文件：

```ini
[common]
mode = remote
crypt = 125 ; 确保这个密钥与您的客户端配置一致

[remote]
port_ws_svr = 10089 ; 容器内部监听的端口
```

然后启动服务：
```bash
docker-compose up -d
```

### 2. 使用 `docker run`

```bash
# 1. 先在当前目录创建一个名为 remote.ini 的配置文件
#    内容同上

# 2. 运行容器
docker run -d \
  --name liuproxy_remote_server \
  -p 10089:10089 \
  -v $(pwd)/remote.ini:/app/configs/remote.ini \
  --restart unless-stopped \
  your-dockerhub-username/liuproxy-remote:latest
```

---

## 在PaaS平台（如Railway）上部署

`liuproxy-remote`可以轻松部署到任何支持Docker的PaaS平台。

1.  **Fork** 本仓库到您自己的GitHub账户。
2.  在Railway上创建一个新项目，并连接到您Fork的仓库。
3.  Railway会自动检测`Dockerfile`并进行构建和部署。
4.  **无需配置`PORT`环境变量**，程序会自动使用Railway提供的端口。
5.  在Railway服务的"Networking"选项卡中，获取您的公共域名（例如 `my-remote.up.railway.app`）。

**客户端配置**:
当您连接部署在PaaS平台上的`remote`服务时，您的`liuproxy-local`客户端**必须**按如下方式配置：
*   **Address**: `my-remote.up.railway.app` (您的公共域名)
*   **Port**: `443`
*   **Scheme**: `wss`

---

## 从源码构建

1.  克隆仓库:
    ```bash
    git clone https://github.com/your-username/liuproxy-remote.git
    cd liuproxy-remote
    ```
2.  构建:
    ```bash
    go build -o liuproxy-remote ./cmd/remote
    ```
3.  运行:
    ```bash
    ./liuproxy-remote --config configs/remote.ini
    ```

---

## 协议兼容性

本项目与 [`liuproxy-gateway`](https://github.com/your-username/liuproxy-gateway) 和 `liuproxy-android-app` 的**Go Remote**模式完全兼容。
// --- END OF NEW FILE ---
```

