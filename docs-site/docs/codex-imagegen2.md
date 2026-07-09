# Codex 生图 Skill 接入

这一页讲的是在 Codex 里使用 `imagegen2` 生图 Skill，让图片生成请求走 subapi 中转站。适合需要在 Codex 对话里直接生成海报、配图、参考图改图的用户。

## 你会得到什么

- 在 Codex 里用 `$imagegen2` 触发生图。
- 使用站内 API Key 走 `https://subapi.loucer.cn/v1`。
- 支持纯文本生图、参考图编辑、1K/2K/4K、常用比例和输出格式。
- 生成成功后图片会保存到本地目录，方便继续发给 Codex 分析或放进项目里。

示例效果：

![imagegen2 生成示例](/images/imagegen2/token-live-selling-2k.png)

## 前置条件

先确认这三件事：

1. subapi 账户有余额或有效订阅。
2. 已经创建 API Key，并且 Key 绑定了支持图片模型的分组。
3. Codex 客户端可以读取本机环境变量。

如果还没有 Key，先看 [API Key 创建与安全](/api-key)。

## 1. 安装 imagegen2 Skill

把 `imagegen2` 文件夹放到 Codex 的全局 skills 目录。

Windows 常见路径：

~~~text
C:\Users\你的用户名\.codex\skills\imagegen2
~~~

目录结构应类似这样：

~~~text
imagegen2/
  SKILL.md
  agents/
    openai.yaml
  scripts/
    imagegen2.py
~~~

安装后重启 Codex，或重新打开当前 Codex 会话，让 Codex 重新加载 skills。

## 2. 配置中转站地址和 API Key

`imagegen2` 默认走 subapi 的 OpenAI 兼容地址：

~~~text
https://subapi.loucer.cn/v1
~~~

建议用环境变量配置 API Key，不要把 Key 写进聊天内容、截图或文档里。

Windows PowerShell：

~~~powershell
setx IMAGEGEN2_API_KEY "sk-你的站内密钥"
setx IMAGEGEN2_BASE_URL "https://subapi.loucer.cn/v1"
~~~

macOS / Linux：

~~~bash
export IMAGEGEN2_API_KEY="sk-你的站内密钥"
export IMAGEGEN2_BASE_URL="https://subapi.loucer.cn/v1"
~~~

设置后重新打开终端和 Codex。

::: warning 不要泄露 Key
API Key 只放在本机环境变量里。不要把完整 Key 发给别人，也不要放到公开仓库。
:::

## 3. 纯文本生图

在 Codex 里可以直接这样说：

~~~text
[$imagegen2] 帮我生成一张：中转站 Token 滞销了，直播卖货风格，要求 2K
~~~

如果你想在终端里测试脚本，可以运行：

~~~bash
python C:/Users/你的用户名/.codex/skills/imagegen2/scripts/imagegen2.py \
  --prompt "中转站 Token 滞销了，直播卖货风格" \
  --model gpt-image-2 \
  --ratio 1:1 \
  --resolution 2K \
  --quality medium
~~~

成功后会输出保存路径，例如：

~~~text
output/imagegen2/imagegen2-20260709-083937-1.png
~~~

## 4. 参考图 / 编辑图

如果要带参考图，让 Codex 按某张图继续改，可以附上图片路径：

~~~bash
python C:/Users/你的用户名/.codex/skills/imagegen2/scripts/imagegen2.py \
  --prompt "保留主体姿势，改成直播间带货海报风格" \
  --image C:/Users/你的用户名/Pictures/reference.png \
  --model gpt-image-1.5 \
  --ratio 1:1 \
  --resolution 1K
~~~

在 Codex 对话里也可以直接说：

~~~text
[$imagegen2] 参考这张图，改成中转站 Token 直播卖货海报风格，1:1，1K
~~~

然后把参考图拖进 Codex，或提供本地图片路径。

## 5. 常用参数

| 参数 | 可选值 | 说明 |
| --- | --- | --- |
| `--model` | `gpt-image-2` / `gpt-image-1.5` / `gpt-image-1` | 默认 `gpt-image-2` |
| `--ratio` | `auto` / `1:1` / `16:9` / `4:3` / `3:4` / `9:16` | 建议明确指定比例 |
| `--resolution` | `1K` / `2K` / `4K` | 分辨率越高，耗时和费用通常越高 |
| `--quality` | `low` / `medium` / `high` / `auto` | 默认 `medium` |
| `--background` | `auto` / `transparent` | 透明背景会自动切到兼容模型 |
| `--output-format` | `png` / `webp` / `jpeg` | 默认 `png` |
| `--n` | 数字 | 生成张数，默认 1 |

常见尺寸映射：

| 比例 + 清晰度 | 典型尺寸 |
| --- | --- |
| `1:1 + 1K` | `1024x1024` |
| `1:1 + 2K` | `2048x2048` |
| `16:9 + 1K` | `1536x864` |
| `9:16 + 1K` | `864x1536` |

## 6. 怎么确认走的是 subapi

生成后回到 subapi 的 **使用记录** 查看：

- 有图片模型调用记录：说明请求已经走 subapi。
- 没有记录：优先检查 `IMAGEGEN2_BASE_URL`、`IMAGEGEN2_API_KEY` 和 Codex 是否重启。

## 常见问题

### 为什么 `gpt-image-2` 偶尔失败？

如果出现 `502`、`524` 或超时，一般是上游图片账号响应慢或暂时不可用，不代表本地 Skill 配错。可以稍后重试，或临时换 `gpt-image-1.5`。

### 为什么 1K 和 2K 费用不一样？

图片模型通常会按模型、尺寸、质量、张数计费。`2K` 的像素量更大，费用高于 `1K` 是正常现象。

### 为什么建议明确写比例和清晰度？

如果使用 `auto`，上游可能按默认规格出图，费用和尺寸不够稳定。想控制成本时，建议明确写：

~~~text
1:1，1K，quality medium
~~~

需要更高清时再改成：

~~~text
1:1，2K，quality medium
~~~

### 透明背景怎么用？

使用：

~~~bash
--background transparent
~~~

如果当前模型不支持透明背景，脚本会自动切到兼容模型，避免请求直接失败。
