-- gemini / antigravity 分组的生图开关同时约束 Gemini 系入口，存量分组统一置为开启。
-- composite 分组的同一开关还约束 OpenAI 生图，不在本迁移范围内。
-- 测试站明确保留以下四个 Gemini 分组当前的关闭状态。
UPDATE groups
SET allow_image_generation = true
WHERE platform IN ('gemini', 'antigravity')
  AND allow_image_generation = false
  AND NOT (
      platform = 'gemini'
      AND name IN (
          'Gemini',
          'Gemini-4-preview 专用分组',
          'Gemini-尊享分组',
          'Gemini-活动分组'
      )
  );
