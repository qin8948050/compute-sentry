# Build Stage for Spy Library
FROM m.daocloud.io/docker.io/gcc:12 AS spy-builder
RUN apt-get update && apt-get install -y cmake
COPY ./spy /app/spy
WORKDIR /app/spy
RUN mkdir -p build && cd build && rm -rf * && cmake .. && make

# Build Stage for Agent
FROM m.daocloud.io/docker.io/golang:1.25 AS agent-builder
WORKDIR /app

# 复制 agent 模块相关文件
COPY ./agent/go.mod ./agent/go.sum ./agent/

# 复制 operator 模块（因为 agent 依赖它，通过 replace 指令）
COPY ./operator ./operator/ 

WORKDIR /app/agent
RUN go mod download
COPY ./agent .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent main.go

# Final Image
FROM m.daocloud.io/docker.io/debian:bookworm-slim
WORKDIR /app
COPY --from=spy-builder /app/spy/build/lib/libcompute-sentry-spy.so /app/lib/
COPY --from=agent-builder /app/agent/agent /app/bin/
COPY ./scripts/precheck.sh /app/bin/precheck.sh

# Set defaults
ENTRYPOINT ["/app/bin/agent"]
CMD ["--lib-src", "/app/lib/libcompute-sentry-spy.so", "--lib-dest", "/opt/compute-sentry/lib/libcompute-sentry-spy.so", "--script-src", "/app/bin/precheck.sh", "--script-dest", "/opt/compute-sentry/bin/precheck.sh"]
