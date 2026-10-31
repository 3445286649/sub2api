# 视频参考素材与自定义导航配置

本次只修改 SubAPI；独立画布仍需要消费能力接口并解除自身九图限制，后端升级不会自动改变画布上传界面。

## 上游账号配置

在创建或编辑 OpenAI API Key 账号时启用「视频任务 (/v1/videos)」，随后在「视频模型素材能力」添加规则：

- `model` 填账号模型映射右侧的实际上游模型名称；`*` 是本账号的默认规则。精确规则优先。
- 参考图片最多 30 张，参考视频和音频各最多 10 段。填写 0 表示该素材不支持。
- 勾选支持的分辨率及首尾帧能力；时长填写秒数，0 表示不额外收紧平台的 0–600 秒范围（0 交给上游默认）。配置了任一时长限制时，请求必须明确提交 duration。
- 未配置规则的账号沿用九图和 480p/720p/1080p；参考视频/音频和首尾帧延续原字段透传能力，上游仍可拒绝不支持的素材。
- 九图型号设置 9，三十图型号设置 30；这些限制由实际账号规则决定，不根据模型名猜测。
- 修改账号规则不会改写已经创建任务的归属和价格快照。

规则持久化在既有账号 credentials 的 `video_model_capabilities` 数组，无数据库表结构迁移。例如：

```json
[
  {
    "model": "官转SD2.5-30图版-720P",
    "max_reference_images": 30,
    "max_reference_videos": 10,
    "max_reference_audios": 10,
    "supports_frames": true,
    "resolutions": ["720p"],
    "min_duration": 1,
    "max_duration": 15
  }
]
```

这里的时长和素材能力为配置示例，实际数值须按上游合同或文档填写。

## 视频 API

使用本站 API Key，通过 `Authorization: Bearer <key>` 调用 `POST /v1/videos`。

```json
{
  "model": "官转SD2.5-30图版-720P",
  "prompt": "参考这些分镜，生成连贯的小猫玩毛线视频",
  "duration": 5,
  "ratio": "16:9",
  "resolution": "720p",
  "reference_images": [
    {"image_url": "https://assets.example.com/frame-01.png"},
    {"image_url": "https://assets.example.com/frame-02.png"}
  ]
}
```

`reference_images` 最多 30 项，每项也可以是 URL 字符串。`reference_videos` 和 `reference_audios` 分别使用 `video_url` 和 `audio_url` 对象，或 URL 字符串，各最多 10 项。参考素材存在时允许省略文字提示。后端保留素材顺序、对象扩展字段和其他上游参数，不裁剪素材。

首尾帧使用 `first_frame`、`last_frame`，可填 URL 或 `{"image_url":"https://..."}`；尾帧必须带首帧，首尾帧与参考图片/视频/音频模式不能混用。上游是否接受图片、视频、音频的混合参考及音频时长要求，由上游实际协议决定；本站不下载远程视频或音频来探测元数据。

所有素材需要上游能读取的公网 HTTP(S) 文件地址，不接受本地路径、data URL 或私有 IP 字面量。后端的 URL 格式检查不代表目标域名一定可用，也不保证上游能读取文件。已有图片上传接口仍每次上传一张，限 PNG/JPEG/WebP、5 MB、8192 边长、3200 万像素；本轮没有新增视频/音频文件上传接口。

创建成功后沿用 `GET /v1/videos/{id}` 查询及 `GET /v1/videos/{id}/content` 下载。未完成或失败不扣完成费用；重复查询/下载共用现有防重逻辑。创建请求超时后不可盲目重提付费任务。

## 能力发现

`GET /v1/video-capabilities?model=<URL编码的公开模型名>` 使用相同 API Key。返回当前绑定分组中可调度账号的能力组合：

```json
{
  "model": "官转SD2.5-30图版-720P",
  "profiles": [
    {
      "model": "",
      "max_reference_images": 30,
      "max_reference_videos": 10,
      "max_reference_audios": 10,
      "supports_frames": true,
      "resolutions": ["720p"],
      "min_duration": 1,
      "max_duration": 15
    }
  ]
}
```

不返回上游账号 ID、凭据或内部模型别名。不同账号的 profile 分开返回；前端必须选取一个完整组合，不能把 A 账号的 30 图能力和 B 账号的 4K 能力拼成一个不存在的组合。实际可用性仍以创建时调度和后端校验为准。

## 定价

OpenAI 视频任务使用分组或渠道的显式「按次」或「按秒」定价。4K 档位标记为 `4k`，不会规范化成 480p；既有默认价格仍可作为未命中层级时的明确通用价格。

- 按次：单价 × 任务数 × 创建时倍率。
- 按秒：单价 × 提交秒数 × 任务数 × 创建时倍率，必须显式传整数 `duration`。
- 创建保存价格快照；后续修改价格不会重算已创建任务。
- OpenAI 视频的时长不会套用 Grok 的 15 秒截断；Grok 的既有档位与时长规范化保持独立。

## 自定义侧栏入口

设置 → 自定义菜单页面，填写名称、URL、图标、可见角色和排序，并选择：

- 站内打开：保留 SubAPI 导航，以 iframe 展示目标网页。
- 新标签页打开：点击左侧菜单直接打开新标签页。
- 当前标签页跳转：当前页面离开 SubAPI 并跳转到目标网址。

启用开关关闭时隐藏入口，保留设置。旧菜单缺少新字段时保持启用、站内打开；Markdown 菜单只支持原有站内显示。

新建或已明确选择打开方式的菜单直接使用配置 URL，不自动拼接本站登录 Token、用户 ID 或来源页面。旧菜单缺少 `open_mode` 时保留原有登录参数集成，设置页显示兼容提示；选择打开方式即迁移为安全的普通链接。**旧集成仍有凭据通过 URL 传给目标网站的风险，只应指向可信网站；迁移前确认依赖并改为专用认证。** 本次未改充值等专用嵌入模块的参数逻辑。

目标网站需自行允许 `codeplan.codes` 和 `subapi.loucer.cn` 嵌入；第三方 `X-Frame-Options` / CSP 拒绝嵌入时请改用新标签页。HTTPS 页面无法嵌入不安全的 HTTP 页面，生产配置应使用 HTTPS。URL 不可包含用户名密码，外链使用 `noopener noreferrer`，iframe 使用 sandbox 和不发送 Referer 的策略。

## 发布边界

本轮没有部署、改线上配置或付费生成。后续上线需用目标站正确的画布构建入口；如果画布尚未适配，API 可用不代表画布能选 30 图。

蓝绿切流应统一双域名和视频查询入口：新版本创建的按秒快照不能交给旧版本完成计费（旧版本不识别 `per_second`）。回滚前应处理本次新版创建的在途视频任务，避免旧版本按次结算。
