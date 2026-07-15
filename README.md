# OpenLink 🔗

> ⚠️ **学习研究项目，非生产用途**
>
> 本项目是作者为**研究底层 Agent 工作原理**而创建的个人学习项目，代码结构和实现均以探索为目的，**不适合用于生产环境**。
>
> **目前实测效果并不理想**：网页版 AI 对工具调用的支持参差不齐，稳定性和准确性均有较大局限，距离实用仍有差距。
>
> OpenLink 通过浏览器扩展模拟用户操作来驱动网页 AI，**并不是一个 API 接口**，不适合作为日常 API 调用使用。请合理使用，勿滥用。

---

## 📖 这是什么

**OpenLink 让网页版 AI（Gemini、AI Studio、DeepSeek）直接访问你的本地文件系统、执行命令、读写文件。**

一句话原理：

```
🤖 AI 网页 → 输出 <tool> 指令 → 🧩 Chrome 扩展拦截 → 🖥️ 本地 Go 服务执行 → 结果返回 AI
```

> 💡 **关键点**：OpenLink **不调用任何服务端大模型**。AI 运行在网页端（你在浏览器里用的 Gemini / DeepSeek），本地 Go 服务只负责「工具执行 + 存储 + 编排」。所以你**不需要配置任何 API Key**，开箱即用。

---

## 🎯 一分钟了解：它能做什么

- 📂 让 AI 读取 / 写入 / 搜索你电脑上的文件
- 💻 让 AI 在本地执行 Shell 命令（带沙箱与危险命令拦截）
- 🌐 让 AI 抓取网页内容
- 🧠 支持 Skills（本地 Markdown 技能文件，按需加载）
- 🌱 **自进化能力**：AI 在对话中主动记笔记、写知识库、复盘、整理记忆（详见后文）
- 🔍 文件变更审查：AI 改文件前自动备份，你来决定保留还是撤回

---

# 🚀 保姆级安装教程

下面提供**两条路线**，新手走「方式一」，想自己编译源码走「方式二」。

---

## 方式一：一键安装（🌟 推荐新手）

### 第一步：安装本地服务

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/afumu/openlink/main/install.sh | sh
```

**Windows（PowerShell，以管理员身份打开）**

```powershell
irm https://raw.githubusercontent.com/afumu/openlink/main/install.ps1 | iex
```

### 第二步：启动服务

```bash
openlink
```

> ✅ 服务默认监听 `http://127.0.0.1:39527`，启动后会**在终端打印一串认证 URL**，把它复制下来，等下要用。

### 第三步：安装 Chrome 扩展

> ⚠️ Chrome Web Store 版本尚未上线，目前请手动加载（见「方式二」第三步的加载方法，或用 Release 里的 `extension.zip`）。

### 第四步：连接并使用

1. 点击浏览器工具栏的 OpenLink 图标
2. 把终端打印的**认证 URL** 粘贴到「API 地址」输入框，保存
3. 打开 [Gemini](https://gemini.google.com) 或 [AI Studio](https://aistudio.google.com)，点击页面右下角的「🔗 初始化」按钮

🎉 完成！AI 现在可以使用你的本地工具了。

---

## 方式二：从源码编译（🔧 开发者）

> 📌 适合想看源码、改代码、或一键脚本用不了的情况。总共三步：**装环境 → 编译后端 → 编译扩展**。

### 📋 第 0 步：准备环境

| 工具 | 版本要求 | 用途 | 检查命令 |
|------|----------|------|----------|
| **Git** | 任意新版 | 拉取代码 | `git --version` |
| **Go** | **1.23+** | 编译本地服务 | `go version` |
| **Node.js** | **18+** | 编译浏览器扩展 | `node -v` |
| **Chrome** | 任意新版 | 加载扩展 | 浏览器关于页查看 |
| **npm** | 随 Node 自带 | 安装扩展依赖 | `npm -v` |

> 💡 **验证环境**：在命令行依次运行上面的「检查命令」，能正常输出版本号就说明装好了。
>
> - Go 官网：https://go.dev/dl/ （装完记得重启终端让 `go` 命令生效）
> - Node 官网：https://nodejs.org/ （选 LTS 长期支持版即可）

### 🔧 第一步：获取源码并编译本地服务（Go）

```bash
# 1. 拉取代码
git clone https://github.com/afumu/openlink.git
cd openlink

# 2. 编译（生成可执行文件）
#    Windows:
go build -o openlink.exe .\cmd\server\
#    macOS / Linux:
go build -o openlink ./cmd/server/
```

> ✅ 编译成功后会得到 `openlink`（或 `openlink.exe`），这就是本地服务程序。
>
> 💡 **不想编译也能跑**（适合临时试用）：直接 `go run cmd/server/main.go` 即可启动，只是每次都要临时编译、稍慢。

**启动服务：**

```bash
# Windows
.\openlink.exe -dir="C:\你的\工作目录"

# macOS / Linux
./openlink -dir="/你的/工作目录"

# 或者直接运行（未编译时）
go run cmd/server/main.go -dir="/你的/工作目录"
```

> 📌 `-dir` 指定**工作目录**（AI 只能在这个目录里读写文件，这是安全沙箱）。不填则默认当前目录。
>
> 可选参数：
> - `-port=39527` — 端口（默认 39527）
> - `-timeout=60` — 命令超时秒数（默认 60）
>
> 启动后终端会打印**认证 URL**（含 token），复制备用。

### 🧩 第二步：把项目编译成 Chrome 扩展程序（🌟 重点）

Chrome 扩展不是「一键双击安装」的东西，而是**用前端工具链（Vite）打包出一个文件夹，再手动加载进 Chrome**。步骤如下：

```bash
# 1. 进入扩展目录
cd extension

# 2. 安装依赖（只需第一次，或 package-lock.json 变化时）
npm install
#    💡 如果你更习惯 pnpm，也可以用：pnpm install

# 3. 编译打包
npm run build
```

> ✅ 编译成功后，产物在 **`extension/dist/`** 目录，里面包含：
> | 文件 | 作用 |
> |------|------|
> | `manifest.json` | 扩展清单（Chrome 靠它识别） |
> | `background.js` | 后台 Service Worker |
> | `content.js` | 内容脚本（注入网页、拦截工具调用） |
> | `injected.js` | 注入页面脚本 |
> | `popup.html` / `popup.js` | 点击图标弹出的设置界面 |
> | `assets/` | 样式等静态资源 |

> 💡 **开发模式（可选）**：改了扩展代码后不想反复手动 build，可用 `npm run dev`，它会监听文件改动自动重新打包。

**把编译好的扩展加载进 Chrome：**

1. 打开 Chrome，地址栏输入 `chrome://extensions/` 并回车
2. 右上角打开 **「开发者模式」(Developer mode)** 开关
3. 点击左上角 **「加载已解压的扩展程序」(Load unpacked)**
4. 在弹出的文件选择框里，**选中 `openlink/extension/dist` 这个文件夹**（不是父目录，也不是 zip）
5. 加载成功后，工具栏会出现 OpenLink 图标 ✅

> ⚠️ **常见坑**：
> - 必须加载 `dist/` 目录，而不是 `extension/` 源码目录（没编译的话 Chrome 不认）。
> - 如果 `dist/` 不存在，说明你**漏了 `npm run build` 这一步**。
> - 改了扩展代码后，回到 `chrome://extensions/` 点击该扩展卡片上的 **「刷新」(⟳)** 才能生效。
> - 想打包分发？在 `chrome://extensions/` 里点「打包扩展程序」即可生成 `.crx` / `.pem`。

### 🔗 第三步：连接扩展与服务

1. 点击浏览器工具栏的 OpenLink 图标，弹出设置面板
2. 把第一步启动服务时终端打印的**认证 URL** 粘贴到「API 地址」输入框
3. 点击保存

### ▶️ 第四步：开始使用

访问 [Google AI Studio](https://aistudio.google.com)（🌟 推荐，原生支持系统提示词，最稳定）或 [Gemini](https://gemini.google.com)，点击页面右下角的「🔗 初始化」按钮，AI 即可开始使用本地工具。

> 📌 **覆盖已安装版本**：如果你之前用一键脚本装过 `openlink`，编译后把新产物覆盖到安装目录即可：
> ```powershell
> # Windows
> Copy-Item ".\openlink.exe" "$env:USERPROFILE\.openlink\openlink.exe" -Force
> # macOS / Linux
> cp ./openlink ~/.openlink/openlink
> ```
> 覆盖后直接运行 `openlink` 即启动新版本。

---

## ⭐ 推荐平台

> 🌟 **目前测试效果最佳的平台是 [Google AI Studio](https://aistudio.google.com)**
>
> AI Studio 原生支持配置系统提示词（System Instructions），点击「🔗 初始化」后会自动将工具说明写入系统提示词，无需占用对话上下文，工具调用更稳定、更准确。
>
> 其他平台通过对话消息注入提示词，效果因模型而异。

### 支持的 AI 平台

| 平台 | 状态 | 备注 |
|------|------|------|
| Google AI Studio | ✅ | 🌟 推荐，原生支持系统提示词 |
| Google Gemini | ✅ | |
| DeepSeek | ✅ | |

---

## 🛠️ 可用工具

AI 通过输出 `<tool>` 指令来调用这些能力：

| 工具 | 说明 |
|------|------|
| `exec_cmd` | 执行 Shell 命令 |
| `list_dir` | 列出目录内容 |
| `read_file` | 读取文件内容（支持分页） |
| `write_file` | 写入文件内容（支持追加/覆盖） |
| `glob` | 按文件名模式搜索文件 |
| `grep` | 正则搜索文件内容 |
| `edit` | 精确替换文件中的字符串 |
| `web_fetch` | 获取网页内容 |
| `question` | 向用户提问并等待回答 |
| `skill` | 加载自定义 Skill |
| `todo_write` | 写入待办事项 |
| `memory_write` / `memory_read` | 写入 / 读取记忆（核心 + 天级） |
| `knowledge_write` / `knowledge_read` | 写入 / 读取知识库 |
| `prompt_update` | 修改提示词文件（AGENT/USER/RULE.md） |
| `context_summarize` | 压缩过长上下文并写入天级记忆 |
| `session_log` | 记录会话轮次（驱动空闲检测） |
| `evolution_control` | 自进化总控（见下文） |

---

## ⌨️ 输入框快捷补全

在任意支持的 AI 平台输入框中，OpenLink 提供两种快捷触发：

| 触发方式 | 效果 |
|----------|------|
| 输入 `/` | 弹出当前项目所有 Skills 列表，选择后自动插入工具调用 XML |
| 输入 `@` | 弹出工作目录文件路径补全列表，选择后插入文件路径 |

**操作方式：** ↑ / ↓ 导航 · Enter 确认 · Escape 或点击外部关闭

---

## 🧠 Skills 扩展

Skills 是放在本地的 Markdown 文件，AI 可按需加载，用于扩展特定领域能力（部署流程、代码规范、项目约定等）。

### Skills 目录（按优先级扫描）

OpenLink 依次扫描以下目录，同名 Skill 以先找到的为准：

```
<工作目录>/.skills/
<工作目录>/.openlink/skills/
<工作目录>/.agent/skills/
<工作目录>/.claude/skills/
~/.openlink/skills/
~/.agent/skills/
~/.claude/skills/
```

### 创建 Skill

在任意 Skills 目录下建子目录，放入 `SKILL.md`：

```
.skills/
└── deploy/
    └── SKILL.md
```

`SKILL.md` 格式：

```markdown
---
name: deploy
description: 项目部署流程
---

## 部署步骤
...
```

AI 通过 `skill` 工具加载：

```
<tool name="skill">
  <parameter name="skill">deploy</parameter>
</tool>
```

### 本地技能扫描

除了项目内置 Skills，OpenLink 还扫描以下系统目录：

| 目录 | 来源 |
|------|------|
| `~/.agents/skills/*/SKILL.md` | .agents 全局技能 |
| `~/.cursor/plugins/.../skills/*/SKILL.md` | Cursor superpowers 插件技能 |
| `~/.trae-cn/skills/*/SKILL.md` | Trae 自定义技能 |

**启动本地技能服务器：**

```bash
node local-skills-server.js [端口号]   # 默认端口 3456
```

启动后自动扫描上述目录，输入框输入 `/` 即可看到所有来源的技能。

---

## 🌱 自进化能力（Self-Evolution）

OpenLink 在「工具代理」基础上增加了**四层自进化能力**，让 Agent 在对话中持续积累、整理与优化自身。所有能力都以**工具 + Web 控制台 + 后台定时任务**落地，控制台可一键开关。

> 📌 **架构约定（重要）**：本项目**不调用任何服务端大模型**。AI 在网页端运行，本地 Go 服务只做「工具执行 + 存储 + 编排」。因此「会话复盘」「梦境整理」的语义推理由**网页端 LLM 通过工具完成**：服务端在空闲/定时条件满足时设置待办并推送通知，网页 AI 调用 `evolution_control(review_now/dream_now)` 获取材料、自行推理落盘、再调用 `review_done/dream_done` 收尾；服务端负责变更快照、变更日志、通知与回滚。**无需任何额外配置即可使用。**

### ① 基础记忆与知识维护

AI 在对话中主动把有价值信息沉淀到工作空间：

- **记忆**：`MEMORY.md`（核心记忆，长期有效、精炼，会注入系统提示）与 `memory/YYYY-MM-DD.md`（天级记忆，按天记录）。写入采用**哈希去重**，相同内容不重复落盘。
- **知识库**：`knowledge/<topic>.md`（Markdown 源文件，建议用 `[[主题]]` 或相对链接交叉引用形成知识图谱）。
- **提示词**：`AGENT.md` / `USER.md` / `RULE.md`，可由 AI 在对话中实时修改并加载进系统提示。

对应工具：`memory_write`、`memory_read`、`knowledge_write`、`knowledge_read`、`prompt_update`。写记忆/提示词前自动建快照，可用「文件变更审查」面板撤销。

### ② 上下文智能总结

对话历史过长、逼近上下文窗口时，调用 `context_summarize`：对历史轮次做确定性压缩（保留首尾、去冗余）并写入天级记忆，返回可注入的精简上下文，让模型丢掉原始细节仍能接住来龙去脉。

### ③ 会话后主动复盘

一段对话告一段落、进入空闲（默认空闲 10 分钟且会话轮次 ≥ 6）时，服务端请求一次复盘（设待办 + 推送通知），随后由**网页端 LLM 驱动**完成：

- 调用 `evolution_control(review_now)` 获取本次会话材料(brief：会话记录、技能、核心记忆、知识主题)
- 把可复用流程**固化为新技能**（写入 `.skills/<name>/SKILL.md`）
- 修复在用技能中暴露的问题（修改已有 `SKILL.md`）
- 记录未完成任务、把值得长期记住的内容写入记忆
- 调用 `evolution_control(review_done)` 收尾：服务端比对快照、**自动备份**相关文件，变更记录持久化于控制台，有改动才通知，**未变更则静默**

对应工具：`session_log`（记录轮次，驱动空闲检测）、`evolution_control`（enable/disable/review_now/review_done/dream_now/dream_done/status）。

### ④ 梦境记忆整理（Deep Dream）

每日夜间定时任务（默认 **23:55**）请求（当天无新天级记忆则跳过），也可控制台手动触发。语义推理由**网页端 LLM** 完成：

- 调用 `evolution_control(dream_now)` 获取材料(brief：核心记忆 + 近期天级记忆)
- 蒸馏核心记忆：去重、合并、修剪、以新换旧，控制在约 50 条以内，写回 `MEMORY.md`
- 生成叙事风格**梦境日记**，保存于 `memory/dreams/YYYY-MM-DD.md`
- 调用 `evolution_control(dream_done)` 收尾：当天无新日记则跳过、无变化绝不空覆写；记忆均做哈希去重，跨天不重复写入

### 🖥️ 配置与控制台

启动后新增 **Web 控制台**：`http://127.0.0.1:<port>/console?token=<token>`（token 同扩展认证 token）。

控制台可：

- 开/关自进化、调整**空闲触发时间**与**复盘轮次阈值**
- 查看**自主进化记录**与待处理通知
- 手动获取「复盘 / 梦境材料」（返回 brief，粘贴给网页 AI 执行）
- 速览核心记忆与今日天级记忆

> 💡 由于 LLM 运行在网页端，本项目**无需任何 LLM / API Key 配置**即可使用全部自进化能力。配置保存在 `~/.openlink/config.json`（或工作目录 `.openlink/config.json`）。

### 🛡️ 安全可控

- **隔离执行**：复盘为独立异步任务，工具集收敛，不污染主对话
- **变更可回滚**：改动前自动备份至 `.openlink/backups/`，可结合审查机制还原
- **变更可追溯**：每次自主改动持久化于 `.openlink/evolution-log.json`，控制台可查
- **未变更不打扰**：有改动才推送通知，否则静默

---

## 🔍 文件变更审查

AI 执行 `write_file` 或 `edit` 修改本地文件时，OpenLink 会自动创建文件快照，操作完成后弹出审查面板，让你决定是否保留变更。

### 审查面板操作

| 按钮 | 说明 |
|------|------|
| 全部保留 | 保留本次任务所有被修改的文件 |
| 保留选中 | 只保留勾选的文件，撤回未勾选的 |
| 撤回选中 | 将勾选的文件恢复到修改前状态 |
| 全部撤回 | 将所有文件恢复到修改前状态 |

### 审查 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/review` | 获取待审查的文件变更列表 |
| POST | `/review/undo` | 撤回文件变更（`{"path":"..."}` 或 `{}` 撤回全部） |
| POST | `/review/keep` | 保留文件变更（`{"paths":[...]}` 或 `{}` 保留全部） |

---

## 🛡️ 安全机制

- **沙箱隔离**：所有文件操作限制在指定工作目录（`-dir`）内
- **危险命令拦截**：`rm -rf`、`sudo`、`curl` 等命令被屏蔽
- **超时控制**：命令执行默认 60 秒超时

---

## ⚙️ 命令行参数

```bash
openlink [选项]

选项：
  -dir string    工作目录（默认：当前目录）
  -port int      监听端口（默认：39527）
  -timeout int   命令超时秒数（默认：60）
```

---

## ❓ 常见问题（FAQ）

**Q：`npm run build` 报错 `npm: command not found`？**
A：说明没装 Node.js。去 https://nodejs.org/ 装 LTS 版，装完重启终端再试。

**Q：加载扩展时 Chrome 提示「清单文件缺失或不可读」？**
A：你选错了文件夹。必须选编译产物 **`extension/dist`** 目录，且里面要有 `manifest.json`。

**Q：扩展图标点了没反应 / 连不上服务？**
A：检查①是否已开启「开发者模式」；②「API 地址」是否填了启动时的**完整认证 URL**（含 `?token=`）；③服务是否还在运行（`go run` 关闭终端就停了）。

**Q：`go build` 报错版本不对？**
A：Go 要求 **1.23+**。运行 `go version` 确认，太低就去 https://go.dev/dl/ 升级。

**Q：改了扩展代码后没生效？**
A：回到 `chrome://extensions/`，点该扩展卡片上的 **⟳ 刷新** 按钮重新加载。

---

## 📚 更多文档

- 🔧 开发指南（项目结构、添加平台/工具、发布）：[docs/development.md](docs/development.md)

---

## 💬 群交流

加微信：**afumudev**，备注：**openlink**

---

## 🙏 致谢

本项目在开发过程中参考了以下优秀的开源项目：

- [opencode](https://github.com/anomalyco/opencode)
- [MCP-SuperAssistant](https://github.com/srbhptl39/MCP-SuperAssistant)
- [learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)

感谢这些项目的作者和贡献者。

---

## ⚠️ 免责声明

本项目仅供学习和研究使用，**严禁用于任何商业用途**。
