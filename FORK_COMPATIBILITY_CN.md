# GentlemanHu Fork：CLIProxyAPI + agy 兼容修复说明

基线：`adeebahmad01/cliproxy-antigravity` v0.3.2。维护分支：`fix/oauth-model-alias-and-claude-bridge`。

## 本次修复

1. **OAuth 账号模型发布**：插件从仅支持静态模型改为 `executor_model_scope: oauth`，实现 `model.for_auth`。使 CLIProxyAPI 能对每个启用的 `agy` 凭据应用 `oauth.model-alias.agy`、`fork`、`force-mapping` 与独立额度/冷却调度。旧版只发布静态目录，出现别名已保存却返回 `400 unknown provider for model` 的现象。
2. **按账号 HOME 发现模型**：`agy models` 在选定账号的 `HOME` / `USERPROFILE` 中运行，缓存按账号隔离。慢速 `agy models` 不持有全局模型缓存锁。
3. **安全的元数据最小化**：新增可选 `strip_client_metadata`，默认 `false`。启用后，仅清除 System/Developer 内容中独立成行的 `x-anthropic-billing-header:`、`anthropic-version:` 和 `anthropic-beta:`，不改写用户内容、Claude Agent SDK 身份指令或工具定义。这是减少无关元数据，不是保证规避第三方服务限制。
4. **回归测试**：模拟双账号独立模型发现、OAuth 执行范围、模型路由、凭据 HOME 错误、请求文本保真与配置合法性。

## 必须知道

- **需使用带动态插件支持的 CLIProxyAPI**。建议使用当前支持 OAuth 插件模型隔离的 v8.0.79 或更新版本；不适用于 `no-plugin` 构建。
- **必须安装官方 `agy` CLI**，并使用该 CLI 为每个独立 HOME 登录。
- `agy` 多账号的 macOS Keychain 隔离需要在真实机器上复核；独立 HOME 并不必然代表不同系统 Keychain 凭据。
- 别名只改“对客户端显示的模型 ID”，例如把 Gemini Flash 显示为 `claude-opus-4-7-anti`，并不改变底层模型。模型上下文和能力以 `agy` 实际返回为准。
- Claude Code `/v1/messages` 经 CLIProxyAPI 转换为 Chat Completions；本插件的工具调用是 Prompt/JSON 桥接而非原生 Anthropic 无损协议。复杂 MCP / 子代理 / 跨账号会话恢复需要真实验收。
- Google 消费者订阅对把 `agy` 接入第三方 Agent 的使用存在服务条款限制。即使技术成功，也不代表获得 Google 官方授权。
- `curl: (52) Empty reply from server` 是 HTTP 连接未收到完整响应，与“别名未注册”不同。需先确认 8317 监听进程、无认证请求的 HTTP 返回以及 macOS 崩溃报告，不应该假定本补丁必然解决 52。

## 示例配置（保留原有其他 Provider）

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    cliproxy-antigravity:
      enabled: true
      binary_path: "agy"
      accounts_dir: "~/.agy-accounts"
      limits_tracking: true
      limits_refresh_interval: "1m"
      enforce_limits: false
      strip_client_metadata: false

oauth:
  model-alias:
    agy:
      - name: "gemini-3.8-flash-high"
        alias: "gemini-flash-anti"
        fork: true
        force-mapping: true
```

示例中的 `oauth` 是已有配置的顶层映射，**不要覆盖原有 `oauth.auth-dir` / `oauth.providers`**。多个 `oauth.model-alias` 通道共存时只增添 `agy`。

在管理页面 **Antigravity accounts** 通过官方 `agy` 登录后，应能看到至少一份 `type: "agy"` 的已启用认证记录。没有此记录时，OAuth 模型别名无法被关联到认证账号。

### 验证

```bash
export CPA_KEY="your-CLIProxyAPI-client-key"
curl --noproxy '*' -fsS http://127.0.0.1:8317/v1/models \
  -H "Authorization: Bearer $CPA_KEY" \
  | jq -r '.data[].id' | grep -E 'gemini-3.8-flash-high|gemini-flash-anti'
```

理想状态应该同时列出原始模型和别名。若返回 `curl (52)`，先诊断 CLIProxyAPI 进程或代理，不能用结果判断别名。

```bash
curl --noproxy '*' -sS http://127.0.0.1:8317/v1/chat/completions \
  -H "Authorization: Bearer $CPA_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gemini-flash-anti","messages":[{"role":"user","content":"只回复OK"}],"stream":false}'
```

### 构建

```bash
go test ./...
CGO_ENABLED=1 go build -buildmode=c-shared -o cliproxy-antigravity.dylib .
```

macOS 上使用 `.dylib`；Linux 将目标改为 `.so` 并在相应平台编译，不可跨平台直接复用二进制。Fork 版本应使用本仓库编译出的插件，而非官方插件商店里仍指向上游的 Release。

## 兼容性验收范围

本次单元测试验证代码路径，并不等于已经在你的 Mac、Keychain、Google 账号、Claude Code 与 CLIProxyAPI 全链路实测。测试结果与 Release 状态应以 GitHub Actions 和你机器上的实际请求为准。
