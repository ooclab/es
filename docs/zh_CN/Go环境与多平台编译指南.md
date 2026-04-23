# Go 环境与多平台编译指南

本文档用于在本地用较新的 Go 版本重新构建 `ooclab/es`，并给出与 `ooclab/otunnel` 联调时的建议流程。

## 1. 安装 Go（建议）

建议使用 **Go 1.25+**（本仓库已使用模块化管理）。

### macOS (Homebrew)

```bash
brew update
brew install go
```

### Ubuntu/Debian

```bash
sudo apt-get update
sudo apt-get install -y golang-go
```

### 验证

```bash
go version
go env GOPATH GOMOD
```

## 2. 获取并构建 es

```bash
git clone https://github.com/ooclab/es.git
cd es
go mod tidy
go test ./...
```

> 当前仓库已内置 `github.com/ooclab/es/logger` 日志适配包，不依赖外部日志模块。

## 3. 多平台编译（es）

仓库提供了脚本：

```bash
./scripts/build_multi_platform.sh
```

脚本会对以下目标执行 `go build ./...`：

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64

## 4. 与 otunnel 联调（本地依赖 es）

当 `otunnel` 依赖本地修改版 `es` 时，在 `otunnel/go.mod` 中增加：

```go
replace github.com/ooclab/es => /path/to/your/es
```

然后在 `otunnel` 仓库执行：

```bash
go mod tidy
go test ./...
GOOS=linux GOARCH=amd64 go build ./...
GOOS=linux GOARCH=arm64 go build ./...
GOOS=darwin GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```

## 5. 依赖升级建议

- 先在网络可访问环境中执行：`go get -u ./... && go mod tidy`
- 对核心依赖逐项验证行为兼容性（尤其是日志、网络与加解密链路）
- 升级后执行全量测试与多平台构建再发布
