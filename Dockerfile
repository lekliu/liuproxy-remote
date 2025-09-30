# --- STAGE 1: Build Stage ---
# 使用与原项目一致的Go版本和Alpine基础镜像，确保环境一致性
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 优化依赖缓存：先复制go.mod和go.sum，然后下载依赖
COPY go.mod go.sum ./
RUN export GOPROXY=https://goproxy.cn,direct && go mod download

# 复制所有源代码
COPY . .

# 只编译 remote 一个可执行文件
# 使用 -trimpath 和 -ldflags "-s -w" 来减小最终二进制文件的大小
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/remote_server ./cmd/remote


# --- STAGE 2: Final Image ---
# 使用一个干净、最小化的Alpine镜像作为最终镜像
FROM alpine:latest

# 设置工作目录
WORKDIR /app

# 只从builder阶段复制编译好的二进制文件和默认配置文件
COPY --from=builder /app/bin/remote_server .
COPY configs/remote.ini ./configs/

# 暴露服务端口 (这主要用于文档目的，Railway等平台会忽略它)
EXPOSE 10089

# 设置容器的启动命令
# 程序将读取环境变量PORT，如果不存在，则使用配置文件中的port_ws_svr

# CMD ["./remote_server", "--config", "configs/remote.ini"]
CMD ["sh", "-c", "printenv && ./remote_server --config configs/remote.ini"]