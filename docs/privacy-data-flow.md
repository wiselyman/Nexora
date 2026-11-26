# Nexora 隐私与数据流

最后核对：2026-11-26

这份说明帮助你判断是否适合把 Nexora 连接到 ChatGPT。Nexora 在本机运行，但使用 OpenAI Tunnel 时，工具请求和结果会经过 OpenAI 服务。不要把“本机运行”理解成“所有数据都留在本机”。

## 数据会经过哪里

```text
ChatGPT / OpenAI 产品
        ↕ MCP 请求、工具参数和结果
OpenAI Tunnel 服务
        ↕ tunnel-client 发起的出站 HTTPS 连接
Nexora 浏览器配置页 → http://localhost:18080 → Nexora 本机 UI/API
本机 tunnel-client → http://127.0.0.1:5060/mcp → Nexora MCP
```

根据 [OpenAI tunnel-client 架构说明](https://github.com/openai/tunnel-client/blob/master/docs/architecture.md)，MCP JSON-RPC 请求、工具参数、响应和流事件会经过 OpenAI 产品运行环境、Tunnel 服务及 tunnel-client 的控制连接；只有 tunnel-client 到本机 MCP 服务的最后一段是本地连接。Nexora 不需要开放公网入站端口。

因此，使用对应工具时，下列内容可能进入 ChatGPT/OpenAI 的请求与响应数据流：命令文本及其标准输出/错误、被读取或写入的文件内容、桌面截图、窗口标题、系统信息，以及输入工具的文本。实际调用由已连接的 OpenAI 产品发起。OpenAI 侧的数据处理和保留方式取决于你使用的产品、账户和组织设置；Nexora 无法控制这些设置。

## 凭据路径

- Tunnel 运行密钥由 Nexora 保存到系统凭据库；不可用时写入用户配置目录中的 `secrets.json`。启动 tunnel-client 时密钥通过环境变量传递，不放在命令行参数中。Tunnel 客户端会将运行密钥发给 OpenAI Tunnel 控制面用于认证；它不会作为 MCP 请求头转发给本机 MCP 服务。
- 启动 Tunnel 时，Nexora 不会把当前用户的整个环境交给子进程，只传递必要的系统变量、语言环境，以及官方文档支持的代理与信任配置：`HTTP_PROXY`、`HTTPS_PROXY`、`NO_PROXY`、Tunnel 专用代理变量、`CA_BUNDLE`、`ENTERPRISE_CA_BUNDLE`、系统证书路径和控制面 mTLS 证书/密钥路径。若代理地址包含账号密码，代理变量会按网络配置传给 tunnel-client；请将其视为敏感设置。证书/密钥环境变量只传路径，文件内容不由 5060 读取。
- Nexora 自己的随机 MCP 令牌用于保护本机 `/mcp`。Tunnel 客户端通过 `MCP_EXTRA_HEADERS` 将它加到发往已配置 MCP 服务的请求中。官方 tunnel-client 文档称这类静态头只注入到配置的 MCP 服务 origin，不发送给 OpenAI 控制面或无关主机。MCP 请求和工具数据本身仍会经过 OpenAI。
- Nexora 的界面会提示 ChatGPT 连接器认证选择“无”，由本机令牌完成本地鉴权。这个连接器配置尚未完成真实 ChatGPT 端到端验证，因此上线前仍需核验实际连接器的认证行为。

## 本机权限的影响

- **文件访问**：关闭时不能使用文件工具；开启后只允许访问设置中选定的目录。读取的内容会作为工具结果返回给调用它的 OpenAI 产品。
- **命令执行**：关闭时不能启动命令，MCP 工具列表也会隐藏命令工具；开启后每条完整命令都要在本机 Nexora 窗口单次批准。批准后命令可以访问当前账户本来有权访问的内容，范围可能超出文件目录白名单，并可能修改或删除数据。管理员权限还需要单独开启并通过系统授权。
- **打开网页**：默认关闭。开启后，ChatGPT 可要求 Nexora 在本机默认浏览器打开 HTTP/HTTPS 页面；浏览器可能使用已登录会话或触发下载。关闭时 MCP 工具列表隐藏该工具，服务端仍会拒绝调用。
- **桌面访问**：开启后，截图、窗口信息及键鼠操作可以暴露或改变当前桌面上的内容。截图可能包含通知、个人资料或其他敏感信息。
- 截图在本机限制为最大 32 MiB、每边最多 16,384 像素、最多 32 百万像素，并在返回前验证 PNG 格式。
- 命令、文件、桌面、浏览器和管理员权限默认关闭。请只开启当前需要的能力，并在连接器中仅授予可信账户访问权限。

## 本地保存与清理

设置文件保存在当前用户的系统配置目录下的 `Nexora/config.json`；秘密优先保存在系统凭据库，回退时位于同目录的 `secrets.json`。Unix 回退文件权限为仅当前用户可读写；Windows 依赖用户配置目录 ACL。设置页可在 Tunnel 停止后清除本机保存的运行时 API Key，保留本机 MCP 令牌。此操作不会撤销 OpenAI Platform 上的 Key；如怀疑 Key 泄露，必须在 Platform 单独撤销。若系统凭据库拒绝删除，界面会报告失败：请在系统凭据管理器中删除服务 `Nexora` 的 `openai-tunnel-runtime-key` 条目；若当前账户使用回退文件，请先退出 Nexora，再从 `secrets.json` 删除 `runtime_api_key` 字段并保留 `mcp_token`。运行日志显示在 Nexora 客户端的诊断标签中，Nexora 会对日志中的已配置运行密钥和 MCP 令牌做脱敏。

Nexora「诊断」标签中的“导出诊断”会生成 JSON 文件，直接字段仅包含应用版本、操作系统、架构、连接状态、权限开关、凭据存储类型和最近日志；不会直接加入完整配置、Tunnel ID、主机名或 API 密钥字段。日志进入内存前会对已配置的运行密钥和 MCP 令牌做精确替换，并尝试识别常见的 `Bearer` 凭据、API Key/Token/Auth/密码字段、部分常见服务令牌前缀、带密码的 URL 和 PEM 私钥块。自动规则不能识别所有秘密格式、编码或拆分后的值，也不会清除本机路径和其他个人标识。命令输出会作为用户请求的 MCP 工具结果返回，但不会复制到 5060 的运行日志或诊断包。分享 Nexora 诊断包前请先打开 JSON 检查内容。

卸载时默认保留配置目录和凭据。Unix/Windows 卸载脚本会询问是否彻底删除；确认后会在删除程序前调用已安装的 Nexora，清除系统凭据库中的 `openai-tunnel-runtime-key`、`local-mcp-token` 和用户配置目录。若凭据库拒绝删除，卸载会停止并保留程序，供你在系统凭据管理器中处理后重试。无交互终端时默认保留数据。若登录启动项因内容被修改而保留，卸载脚本也会保留其所有权标记和配置目录，避免丢失识别该启动项所需的信息。无论选择哪种卸载方式，都请先退出 Nexora 和 tunnel-client。设置页的“清除本机保存的 Key”只删除 Tunnel Runtime API Key，不删除本机 MCP 令牌或其他设置。若选择保留数据，也可在之后通过系统凭据管理器或删除配置目录进行手动清理。

## 仍需确认的事项

真实 ChatGPT Tunnel 连接器尚未端到端验收；各操作系统的权限提示、命令进程树取消和安装/卸载流程也尚未完成实机验收。请把当前版本视为早期版本，不要用于无人值守的敏感数据操作。
