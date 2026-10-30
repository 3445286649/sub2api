# 下游生图 API 对接

本文以测试站 `https://usapi.codeplan.codes` 为例。正式使用时将域名换成实际站点域名。所有请求使用用户在本站创建的 API Key；不要将 Key 放进前端代码、图片 URL 或日志。下文的 `$API_KEY` 和模型名均需替换为自己的值。

## 接入前确认

1. 在本站创建 API Key，并绑定允许生图、包含目标模型的分组；聚合 Key 也必须绑定相应分组。
2. 调用 `GET /v1/models` 查看该 Key 可见的模型 ID。模型可见不保证上游账号当前可用；正式接入前用低成本请求验证。
3. 请求中的 `model` 必须使用本站对外模型 ID，不要直接填上游账号内部映射名。图片尺寸、质量、张数等参数仅在目标模型和上游支持时传入。

| 模型类别 | 文生图 | 图生图/编辑 | 结果位置 |
| --- | --- | --- | --- |
| OpenAI `gpt-image-*` | `POST /v1/images/generations` | `POST /v1/images/edits` | `data[].b64_json` 或 `data[].url` |
| Grok `grok-imagine-image*` | `POST /v1/images/generations` | `POST /v1/images/edits` | `data[].b64_json` 或 `data[].url` |
| Gemini 图片模型 | `POST /v1beta/models/{model}:generateContent` | 同一端点，在 `contents[].parts[]` 中加入参考图 | `candidates[].content.parts[].inlineData` |
| Banana 模型/别名 | 与 Gemini 相同 | 与 Gemini 相同 | 与 Gemini 相同 |

`Banana` 是 Gemini 生图模型或站内映射名，不是独立协议。使用本站为 Key 开放的准确模型 ID，例如 `nano-banana-2`；若该模型未在 Key 的模型列表中出现，不能仅凭名称假定可用。Gemini 原生协议使用 `x-goog-api-key`，本站也接受 `Authorization: Bearer`。

## OpenAI 生图

测试站公开模型目录目前包含 `gpt-image-2` 及其分辨率别名。最小文生图请求：

```bash
curl -sS 'https://usapi.codeplan.codes/v1/images/generations' \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-image-2","prompt":"一张白底产品摄影图","n":1}'
```

上传本地参考图进行编辑：

```bash
curl -sS 'https://usapi.codeplan.codes/v1/images/edits' \
  -H "Authorization: Bearer $API_KEY" \
  -F 'model=gpt-image-2' \
  -F 'prompt=保持产品形状，改成蓝色背景' \
  -F 'image=@reference.png'
```

编辑接口也接受 JSON：`{"model":"gpt-image-2","prompt":"...","images":[{"image_url":"https://example.com/reference.png"}]}`；`image_url` 可以是可访问的 HTTPS 图片地址或 `data:image/png;base64,...`。JSON 编辑请求使用 `images` 数组，不能只传单独的 `image` 字段。返回结果应同时兼容 `data[].b64_json` 和 `data[].url`，不要假定一定得到公开图片 URL。

## Grok 生图

Grok 分组沿用相同的本站端点，但要使用该分组提供的 Grok 生图模型 ID。下例的 `grok-imagine-image-quality` 仅是代码支持的模型示例，实际可用性以 Key 的模型列表和上游账号为准。

```bash
curl -sS 'https://usapi.codeplan.codes/v1/images/generations' \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"grok-imagine-image-quality","prompt":"夜晚城市街景","n":1,"response_format":"b64_json"}'
```

Grok 图生图使用 `/v1/images/edits`。JSON 请求的参考图字段为 `image`，值为 `{ "url": "图片地址或 data URL", "type": "image_url" }`：

```bash
curl -sS 'https://usapi.codeplan.codes/v1/images/edits' \
  -H "Authorization: Bearer $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"grok-imagine-image-quality","prompt":"改成水彩风格","image":{"url":"https://example.com/reference.png","type":"image_url"},"response_format":"b64_json"}'
```

本站也接受 Grok 编辑的 `multipart/form-data` `image=@reference.png`，会转换为上游要求的图片对象。请求可以带 `aspect_ratio`、`resolution` 等目标模型支持的选项；不应把 OpenAI 特有参数默认套到 Grok 模型上。

## Gemini / Banana 生图

Gemini 和 Banana 使用 Gemini 原生接口。模型 ID 放在 URL 中，图像输出通过 `generationConfig.responseModalities` 请求；图片以 base64 返回，不是 OpenAI 的 `data[]` 结构。

```bash
MODEL='gemini-2.5-flash-image'  # 换成该 Key 实际可见的 Gemini/Banana 模型 ID
curl -sS "https://usapi.codeplan.codes/v1beta/models/${MODEL}:generateContent" \
  -H "x-goog-api-key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents":[{"role":"user","parts":[{"text":"生成一张白底产品摄影图"}]}],
    "generationConfig":{"responseModalities":["TEXT","IMAGE"],"imageConfig":{"aspectRatio":"1:1"}}
  }'
```

图生图仍调用同一个 `generateContent` 端点，在用户 `parts` 中加入参考图。`data` 填原始图片字节的 base64，**不要**带 `data:image/...;base64,` 前缀：

```json
{
  "contents": [{
    "role": "user",
    "parts": [
      {"text": "保持主体，改为水彩画"},
      {"inlineData": {"mimeType": "image/png", "data": "<BASE64_IMAGE_BYTES>"}}
    ]
  }],
  "generationConfig": {"responseModalities": ["TEXT", "IMAGE"]}
}
```

在响应的 `candidates[].content.parts[]` 中查找 `inlineData`，按它的 `mimeType` 解码 `data`；同一响应也可能包含文字 `text`，不要只读取第一个 part。若使用 `streamGenerateContent?alt=sse`，则需按 SSE 逐段解析。

## 异步与常见错误

OpenAI/Grok 同步生图较慢时，可在站点已开启异步生图对象存储的条件下，调用 `POST /v1/images/generations/async` 或 `POST /v1/images/edits/async`。提交后用**同一把 Key**调用 `GET /v1/images/tasks/{task_id}`，直到 `completed` 或 `failed`；成功图片在 `result.data[].url`。异步接口不适用于 Gemini/Banana，详细状态字段见 [异步生图说明](ASYNC_IMAGE_TASKS.md)。

- `401`：检查 API Key 与认证请求头。
- `400/403`：检查分组是否允许生图、Key 是否绑定目标模型，以及参数是否适用于该模型。
- `404`：检查路径和分组平台；异步接口还需管理员开启对象存储。
- 不要在 HTTP 超时后盲目重试已提交的付费请求；先查站内用量记录或异步任务状态。

本站按实际生效的模型价格、分组/用户倍率及任务结果计费；接口格式本身不决定单价。具体价格以站内模型广场与用量记录为准。
