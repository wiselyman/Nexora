# 验证 Nexora 发布包

GitHub tag 发布会附带跨平台二进制和安装包的 SHA256 清单、每个二进制对应的 SPDX JSON SBOM，以及 GitHub Sigstore 构建来源证明。来源证明把发布文件的摘要关联到执行构建的仓库和工作流；它证明构建来源和文件完整性，不代表代码已经过独立安全审计。

## 核对 SHA256

先下载发布包和同一版本的 `SHA256SUMS`。**先验证 `SHA256SUMS` 的来源证明，再用它检查下载文件**；否则攻击者若同时替换文件和清单，单独比对摘要不能证明来源可信。验证来源证明的方法见下节。之后在解压目录中运行：

```sh
# Linux
sha256sum -c SHA256SUMS

# macOS
shasum -a 256 -c SHA256SUMS
```

Windows PowerShell 可逐个核对：

```powershell
Get-FileHash .\Nexora-<版本>-windows-amd64.zip -Algorithm SHA256
```

将输出的 `Hash` 与 `SHA256SUMS` 中对应文件比较。

## 核对构建来源证明

安装 [GitHub CLI](https://cli.github.com/) 后，运行：

```sh
gh attestation verify ./Nexora-<版本>-darwin-arm64.tar.gz -R <所有者>/<仓库>
```

先验证校验和清单：

```sh
gh attestation verify ./SHA256SUMS -R <所有者>/<仓库>
```

再将示例中的文件名替换为下载的二进制或归档，将 `<所有者>/<仓库>` 替换成 Nexora 的 GitHub 仓库路径。命令应确认 attestation 的来源仓库和发布工作流符合预期。对 Windows ZIP、Linux TAR、二进制或 SBOM 也可用同一命令验证。

## SBOM

每个可执行二进制均有同名 `.spdx.json` 文件，列出 Syft 从二进制识别到的 Go 模块。要让验证结果与下载文件关联，可对二进制及其 SBOM 文件分别运行 `gh attestation verify`；SBOM 本身也包含在发布来源证明的 subject 列表中。

不要只凭文件名判断下载来源。SHA256、Sigstore attestation 和 SBOM 是互补信息；它们不能代替在可信来源下载、代码审查和目标系统实机验收。
