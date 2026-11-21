# 在 CLIProxyAPI 管理面板直接安装 GentlemanHu Fork

## 不需要本地编译，也不需要先 Merge PR

代码分支：`fix/oauth-model-alias-and-claude-bridge`。
独立的商店索引分支：`plugin-store`。
Release 版本：`v0.3.3-fix`。

### 1. 通过 Actions 发布跨平台安装包

打开 [GitHub Actions / Release](https://github.com/GentlemanHu/cliproxy-antigravity/actions/workflows/release.yml)。

1. 如果 GitHub 提示本 Fork 的工作流尚未启用，先启用 Actions。
2. 选择 **Run workflow**。
3. **Use workflow from** 选择 `fix/oauth-model-alias-and-claude-bridge`；若页面显示 `release_version` 参数，保持 `0.3.3-fix`。若未显示该参数，也可以直接点击运行（工作流含 `0.3.3-fix` 默认值）。
4. 工作流先进行 `gofmt` 检查、Go `-race` 测试，再构建 macOS ARM64/AMD64、Linux AMD64/ARM64、Windows AMD64 动态库，并在 Linux AMD64 运行 ABI 冒烟测试。只有全部成功才会创建 `v0.3.3-fix` Release。
5. 在 [Releases](https://github.com/GentlemanHu/cliproxy-antigravity/releases) 确认存在 `checksums.txt` 及对应平台 ZIP。一般不需要手动下载这些文件，CLIProxyAPI 商店会自动下载并校验。

> GitHub Actions 在新 Fork 中可能默认禁用。仅提交工作流文件**不代表已经执行构建或发布 Release**。

### 2. 在 CLIProxyAPI 中添加私人插件源

CLIProxyAPI 当前支持通过 `plugins.store-sources` 增加外部插件注册表。将以下内容**合并**到现有 `config.yaml`（不要覆盖原有 `plugins.configs` 或 OAuth 配置）：

```yaml
plugins:
  enabled: true
  store-sources:
    - "https://raw.githubusercontent.com/GentlemanHu/cliproxy-antigravity/plugin-store/registry.json"
  configs:
    cliproxy-antigravity:
      enabled: true
      binary_path: "agy"
      accounts_dir: "~/.agy-accounts"
      strip_client_metadata: false
```

原有 `plugins.dir` 应保持不变。配置更改后，完整重启 CLIProxyAPI，再打开管理面板 → 插件商店。

从插件源 **GentlemanHu / plugin-store** 选择 `Antigravity CLI Provider — GentlemanHu Fork`，选择安装 `v0.3.3-fix`。
注意：官方源和私人源中都有 ID 为 `cliproxy-antigravity` 的插件，必须明确选择 **私人源**。

### 3. 如果官方商店里的旧版已经安装

同一个插件 ID 的来源切换可能收到 `plugin_store_source_conflict`。这不是模型别名错误，而是 CLIProxyAPI 阻止将官方插件直接覆盖成未知来源。

先备份现有 `config.yaml`、插件设置和认证目录，然后使用 CLIProxyAPI 的插件管理界面**卸载旧插件本身**，再从自己的插件源安装。不要删除已有 `type: agy` 认证 JSON，不要删除 `~/.agy-accounts`，也不要重置 macOS Keychain。最后核对旧插件设置是否仍被保留。

若出现 `plugin_update_requires_restart`，代表某个正在加载的动态库无法原地覆盖，需要完整重启 CLIProxyAPI；根据安装界面提示处理，不要暴力删除正在使用的共享库。

### 4. 确认版本与模型映射

在已启用的插件列表检查版本应该是 **0.3.3-fix**，然后确认账号管理页至少有一个 `type: agy` 的有效凭据。

```yaml
oauth:
  model-alias:
    agy:
      - name: "gemini-3.8-flash-high"
        alias: "gemini-flash-anti"
        fork: true
        force-mapping: true
```

`oauth.model-alias.agy` 必须**合并到现有的 OAuth 配置**中，不应重复创建顶层 `oauth` 块。配置的模型仍是 Gemini Flash，别名不会把它变成 Claude Opus。

```bash
curl --noproxy '*' -fsS http://127.0.0.1:8317/v1/models \
  -H "Authorization: Bearer $CPA_KEY" \
  | jq -r '.data[].id' | grep -E 'gemini-3.8-flash-high|gemini-flash-anti'
```

注意：这个方案**仍然需要预装并登录官方 `agy` CLI**，GitHub Actions 负责的是插件动态库，并不会把 `agy` CLI 本体打包进去。

完整 Claude Code 工具调用兼容性、macOS 多账号 Keychain 真正隔离，以及此前观察到的 `curl (52)`，仍需单独端到端验证。Google 消费者订阅对通过第三方 Agent 使用 `agy` 也存在服务条款限制。
