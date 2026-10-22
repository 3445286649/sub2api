# Codex 生图 Skill

在 Codex 对话里直接生成图片：纯文字生图、带参考图改图，1K / 2K / 4K，常用比例都支持。图片请求走 CodePlan，费用计入你的账户。

**不用下载任何文件。** 复制一段提示词给 Codex，它会在你的电脑上把 Skill 自动建好。

![imagegen2 生成示例](/images/imagegen2/token-live-selling-2k.png)

## 准备

- CodePlan 账户有余额或有效订阅。
- 一个绑定了**支持图片模型分组**的 API Key，见 [模型与分组怎么选](/models-and-groups#用图片模型)。
- 已经安装 Codex，电脑上有 Python 3。

## 第一步：把 Key 设进环境变量

::: danger 不要把 Key 贴进 Codex 对话
Key 发进对话就会留在聊天记录里。先放进环境变量，下一步的提示词只让 Codex「从环境变量读」。
:::

::: code-group

```powershell [Windows PowerShell]
setx IMAGEGEN2_API_KEY "sk-你的密钥"
```

```bash [macOS / Linux]
echo 'export IMAGEGEN2_API_KEY="sk-你的密钥"' >> ~/.zshrc
source ~/.zshrc
```

:::

设置完**重新打开终端和 Codex**，新的环境变量才会生效。macOS 用 bash 的话，把 `~/.zshrc` 换成 `~/.bashrc`。

## 第二步：复制提示词发给 Codex

在 Codex 里新开一个对话，把下面整段复制进去发送（点代码块右上角可以一键复制）：

```text
帮我新建一个全局 Codex skill，名字叫 imagegen2，用来调用 CodePlan 的 OpenAI 兼容图片接口。
先检查本机是否已有 imagegen skill：有就参考它的目录结构和脚本风格；没有就按 Codex skill 标准
创建 SKILL.md、agents/openai.yaml 和 scripts/imagegen2.py，放在全局 skills 目录下。

接口信息：
- 生图接口：https://codeplan.codes/v1/images/generations
- 编辑接口：https://codeplan.codes/v1/images/edits
- API Key：只从环境变量 IMAGEGEN2_API_KEY 读取，不要写进任何文件；读不到就停下并提示我去设置
- 默认模型：gpt-image-2
- 默认输出目录：当前工作目录下的 output/imagegen2/

功能要求：
1. 只有文字提示时走 generations 接口；带参考图时走 edits 接口（multipart/form-data 上传图片）。
2. 支持这些参数：
   - 张数 n：默认 1
   - 比例：auto、1:1、16:9、4:3、3:4、9:16，默认 auto（不传 size）
   - 清晰度：1K、2K、4K
   - 模型：gpt-image-2、gpt-image-1.5、gpt-image-1
   - 背景：auto、transparent，默认 auto
   - 输出格式：png、webp、jpeg，默认 png
   - quality：low、medium、high、auto，默认 medium
3. 比例和清晰度自动换算成 size：1:1+1K=1024x1024，16:9+1K=1536x864，9:16+1K=864x1536，
   4:3、3:4 同理；2K、4K 按比例等比放大。宽高取 16 的倍数。
   如果接口返回尺寸不支持，自动降到最接近的受支持尺寸重试一次，并告诉我实际用的尺寸。
4. 透明背景：请求 background=transparent；如果当前模型报错不支持，自动改用 gpt-image-1.5 重试，并告诉我换了模型。
5. 脚本只用 Python 标准库（urllib、json、base64 等），不要依赖 requests 或其它第三方包。
6. 报错时用中文说清原因：401 是 Key 无效；403 可能是余额不足或分组不支持图片模型；
   429 是请求太频繁；502、503、524 或超时是上游繁忙，可以稍后重试。
7. 图片保存到输出目录，文件名带时间戳，最后打印每张图的保存路径。
8. 任何输出、日志和报错信息里都不要显示完整 API Key。

完成后：
- 告诉我 skill 的安装路径；
- 用主题「中转站 Token 滞销了，直播卖货风格」跑一次纯文字生图（1:1，1K）；
- 再把第一张图当参考图，跑一次编辑（改成 9:16 竖版海报，1K）；
- 把两张成功的图片展示给我。
```

::: info 这一步会产生费用
最后两次测试会生成两张 1K 图片，费用计入你的账户，和平时生图一样。
:::

## 第三步：验收

Codex 完成后，确认这三点：

1. **告诉了你安装路径**：一般是 `~/.codex/skills/imagegen2`，Windows 上是 `C:\Users\你的用户名\.codex\skills\imagegen2`。
2. **两张测试图都成功了**：一张 1:1、一张 9:16 竖版。
3. **站内有记录**：到 [使用记录](https://codeplan.codes/usage) 能看到两次图片模型的调用。

三点都满足，就装好了。之后**重启 Codex** 或新开一个对话，让它加载这个 Skill。

## 日常怎么用

在 Codex 对话里用 `$imagegen2` 触发：

```text
$imagegen2 生成一张赛博朋克风格的城市夜景，16:9，2K
```

```text
$imagegen2 做一个透明背景的咖啡杯图标，1:1，1K，png
```

```text
$imagegen2 参考这张图，保留人物姿势，改成水彩画风格，1K
```

带参考图时，把图片拖进 Codex，或者直接给出本地图片路径。

## 参数说明

| 参数 | 可选值 | 说明 |
| --- | --- | --- |
| 模型 | `gpt-image-2` / `gpt-image-1.5` / `gpt-image-1` | 默认 `gpt-image-2` |
| 比例 | `auto` / `1:1` / `16:9` / `4:3` / `3:4` / `9:16` | 建议明确指定 |
| 清晰度 | `1K` / `2K` / `4K` | 越高越慢、费用越高 |
| quality | `low` / `medium` / `high` / `auto` | 默认 `medium` |
| 背景 | `auto` / `transparent` | 透明背景不支持时会自动换模型 |
| 格式 | `png` / `webp` / `jpeg` | 默认 `png`；透明背景要用 `png` 或 `webp` |
| 张数 | 数字 | 默认 `1` |

常见尺寸：

| 比例 + 清晰度 | 尺寸 |
| --- | --- |
| `1:1` + `1K` | `1024x1024` |
| `1:1` + `2K` | `2048x2048` |
| `16:9` + `1K` | `1536x864` |
| `9:16` + `1K` | `864x1536` |

## 常见问题

### Codex 说读不到 IMAGEGEN2_API_KEY

环境变量设置后没有生效。**完全关闭终端和 Codex 再重新打开**；Windows 用 `setx` 设置的变量，只对之后新开的窗口有效。

### 为什么偶尔失败？

出现 `502`、`524` 或超时，一般是上游图片服务繁忙，不是 Skill 配错了。稍后重试，或者临时换成 `gpt-image-1.5`。

### 报 403

先看报错文字：余额不足就去充值；如果提示模型不可用，说明 Key 所在分组不支持图片模型，换一个分组的 Key。

### 1K 和 2K 为什么价格不一样？

图片按模型、尺寸、质量和张数计费。2K 的像素量是 1K 的 4 倍，费用更高是正常的。想控制成本，就明确写上 `1K` 和 `quality medium`，不要用 `auto`。

### 怎么确认走的是 CodePlan？

生成后到站内 [使用记录](https://codeplan.codes/usage) 看有没有图片模型的调用记录。

::: details 手动安装（备选）
如果不想让 Codex 自动生成，也可以手动建立目录，结构如下：

```text
~/.codex/skills/imagegen2/
  SKILL.md
  agents/
    openai.yaml
  scripts/
    imagegen2.py
```

三个文件的内容可以让 Codex 按上面的提示词生成后，再复制出来保存。装好后重启 Codex。
:::
