package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const AccountTestModeCandy = "candy"

const candyTestPrompt = "请解下面的糖果取样题。袋子里有三种口味：苹果、桃子、西瓜；糖果只能凭形状区分，不能看口味，取出后不放回。\n\n圆形糖果：苹果 7 颗、桃子 9 颗、西瓜 8 颗。\n五角星糖果：苹果 7 颗、桃子 6 颗、西瓜 4 颗。\n\n需要保证抽出的糖果中，存在两颗形状不同、口味分别为苹果和桃子。请计算最少总数和能保证成功的圆形、五角星配额，并计算以下证明字段：圆形西瓜总数、五角星中不含苹果的最大数量、五角星中不含桃子的最大数量、比最少总数少一颗时的圆形反例配额。\n\n只返回一个 JSON 对象，不要 Markdown、代码围栏或 JSON 后的补充文字。字段必须是整数：minimum_draws、round、star、proof.round_watermelon_capacity、proof.star_capacity_without_apple、proof.star_capacity_without_peach、proof.one_less_round_quota。"

const candyAnswerLimit = 32 << 10
const candyStreamLimit = 64 << 10

type candyCountingReader struct {
	reader io.Reader
	read   int64
}

func (r *candyCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += int64(n)
	return n, err
}

type candyParsedAnswer struct {
	MinimumDraws int `json:"minimum_draws"`
	Round        int `json:"round"`
	Star         int `json:"star"`
}

type candyModelAnswer struct {
	MinimumDraws *int             `json:"minimum_draws"`
	Round        *int             `json:"round"`
	Star         *int             `json:"star"`
	Proof        *candyModelProof `json:"proof"`
}

type candyModelProof struct {
	RoundWatermelonCapacity  *int `json:"round_watermelon_capacity"`
	StarCapacityWithoutApple *int `json:"star_capacity_without_apple"`
	StarCapacityWithoutPeach *int `json:"star_capacity_without_peach"`
	OneLessRoundQuota        *int `json:"one_less_round_quota"`
}

type candyTestResult struct {
	Status       string             `json:"status"`
	ParsedAnswer *candyParsedAnswer `json:"parsed_answer,omitempty"`
	MinimumDraws int                `json:"minimum_draws"`
	Explanation  string             `json:"explanation"`
}

func (s *AccountTestService) processCandyStream(c *gin.Context, stream string, body io.Reader) error {
	counted := &candyCountingReader{reader: io.LimitReader(body, candyStreamLimit+1)}
	reader := bufio.NewReaderSize(counted, 4096)
	var answer strings.Builder
	answerTooLong := false
	seenComplete := false
	appendText := func(value string) {
		if value == "" {
			return
		}
		remaining := candyAnswerLimit - answer.Len()
		if remaining <= 0 {
			answerTooLong = true
			return
		}
		if len(value) > remaining {
			value = value[:remaining]
			for !utf8.ValidString(value) && len(value) > 0 {
				value = value[:len(value)-1]
			}
			answerTooLong = true
		}
		answer.WriteString(value)
		s.sendEvent(c, TestEvent{Type: "content", Text: value})
	}
	complete := func() error {
		if !seenComplete {
			return s.sendErrorAndEnd(c, "Candy stream ended before completion")
		}
		result := scoreCandyAnswer(answer.String(), answerTooLong)
		s.sendEvent(c, TestEvent{Type: "test_complete", Success: true, Data: result})
		return nil
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return s.sendErrorAndEnd(c, "Candy stream read failed")
		}
		if counted.read-int64(reader.Buffered()) > candyStreamLimit {
			return s.sendErrorAndEnd(c, "Candy upstream stream exceeded the size limit")
		}
		line = strings.TrimSpace(line)
		if line == "" || !sseDataPrefix.MatchString(line) {
			if readErr == io.EOF {
				return complete()
			}
			continue
		}
		jsonStr := sseDataPrefix.ReplaceAllString(line, "")
		if jsonStr == "[DONE]" {
			if stream == "chat" || stream == "gemini" {
				seenComplete = true
				return complete()
			}
			if readErr == io.EOF {
				return complete()
			}
			continue
		}
		var data map[string]any
		if decodeErr := json.Unmarshal([]byte(jsonStr), &data); decodeErr != nil {
			if readErr == io.EOF {
				return complete()
			}
			continue
		}
		if stream == "gemini" {
			if errData, ok := data["error"].(map[string]any); ok && errData != nil {
				return s.sendErrorAndEnd(c, "Candy upstream returned an error")
			}
			if response, ok := data["response"].(map[string]any); ok {
				data = response
			}
			if candidates, ok := data["candidates"].([]any); ok {
				for _, rawCandidate := range candidates {
					candidate, _ := rawCandidate.(map[string]any)
					content, _ := candidate["content"].(map[string]any)
					parts, _ := content["parts"].([]any)
					for _, rawPart := range parts {
						part, _ := rawPart.(map[string]any)
						if text, _ := part["text"].(string); text != "" {
							appendText(text)
						}
					}
					if reason, _ := candidate["finishReason"].(string); reason != "" {
						seenComplete = true
					}
				}
			}
			if seenComplete {
				return complete()
			}
			if readErr == io.EOF {
				return complete()
			}
			continue
		}

		eventType, _ := data["type"].(string)
		switch stream {
		case "responses":
			switch eventType {
			case "response.output_text.delta":
				if text, _ := data["delta"].(string); text != "" {
					appendText(text)
				}
			case "response.completed", "response.done":
				seenComplete = true
				return complete()
			case "response.failed", "error":
				return s.sendErrorAndEnd(c, "Candy upstream response failed")
			}
		case "chat":
			if errData, ok := data["error"].(map[string]any); ok && errData != nil {
				return s.sendErrorAndEnd(c, "Candy upstream returned an error")
			}
			choices, _ := data["choices"].([]any)
			for _, rawChoice := range choices {
				choice, _ := rawChoice.(map[string]any)
				if delta, ok := choice["delta"].(map[string]any); ok {
					if text, _ := delta["content"].(string); text != "" {
						appendText(text)
					}
				}
				if finishReason, _ := choice["finish_reason"].(string); finishReason != "" {
					seenComplete = true
				}
			}
			if seenComplete {
				return complete()
			}
		case "claude":
			switch eventType {
			case "content_block_delta":
				if delta, ok := data["delta"].(map[string]any); ok {
					if text, _ := delta["text"].(string); text != "" {
						appendText(text)
					}
				}
			case "message_stop":
				seenComplete = true
				return complete()
			case "error":
				return s.sendErrorAndEnd(c, "Candy upstream returned an error")
			}
		}
		if readErr == io.EOF {
			return complete()
		}
	}
}

func scoreCandyAnswer(raw string, tooLong bool) candyTestResult {
	result := candyTestResult{
		Status:       "unscorable",
		MinimumDraws: 21,
		Explanation:  "无法从模型回复中解析规定的 JSON 答案。",
	}
	if tooLong {
		result.Explanation = "模型回复超过判题长度限制，无法判定。"
		return result
	}
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{") {
		return result
	}
	if hasDuplicateCandyJSONKeys(raw) {
		return result
	}
	var modelAnswer candyModelAnswer
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&modelAnswer); err != nil ||
		modelAnswer.MinimumDraws == nil || modelAnswer.Round == nil || modelAnswer.Star == nil || modelAnswer.Proof == nil ||
		modelAnswer.Proof.RoundWatermelonCapacity == nil || modelAnswer.Proof.StarCapacityWithoutApple == nil ||
		modelAnswer.Proof.StarCapacityWithoutPeach == nil || modelAnswer.Proof.OneLessRoundQuota == nil {
		return result
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return result
	}
	parsed := candyParsedAnswer{MinimumDraws: *modelAnswer.MinimumDraws, Round: *modelAnswer.Round, Star: *modelAnswer.Star}
	result.ParsedAnswer = &parsed
	proof := modelAnswer.Proof
	if parsed.MinimumDraws == 21 && parsed.Round == 9 && parsed.Star == 12 &&
		*proof.RoundWatermelonCapacity == 8 && *proof.StarCapacityWithoutApple == 10 &&
		*proof.StarCapacityWithoutPeach == 11 && *proof.OneLessRoundQuota == 8 {
		result.Status = "correct"
		result.Explanation = "9颗圆形至少含苹果或桃子（圆形西瓜只有8颗）；12颗五角星同时超过不含苹果最多10颗、不含桃子最多11颗，因此一定含苹果和桃子，可以跨形状配对。少一颗时可取8颗圆形西瓜，无法保证目标配对。"
	} else {
		result.Status = "incorrect"
		result.Explanation = "抽取数量、形状配额或可核验的证明数字与标准答案不符。"
	}
	return result
}

func hasDuplicateCandyJSONKeys(raw string) bool {
	decoder := json.NewDecoder(strings.NewReader(raw))
	hasDuplicates, err := scanCandyJSONValue(decoder)
	return err != nil || hasDuplicates
}

func scanCandyJSONValue(decoder *json.Decoder) (bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false, nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		hasDuplicates := false
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return false, fmt.Errorf("invalid JSON object key")
			}
			if _, exists := seen[key]; exists {
				hasDuplicates = true
			}
			seen[key] = struct{}{}
			nestedDuplicates, err := scanCandyJSONValue(decoder)
			if err != nil {
				return false, err
			}
			hasDuplicates = hasDuplicates || nestedDuplicates
		}
		_, err = decoder.Token()
		return hasDuplicates, err
	case '[':
		hasDuplicates := false
		for decoder.More() {
			nestedDuplicates, err := scanCandyJSONValue(decoder)
			if err != nil {
				return false, err
			}
			hasDuplicates = hasDuplicates || nestedDuplicates
		}
		_, err = decoder.Token()
		return hasDuplicates, err
	default:
		return false, nil
	}
}
