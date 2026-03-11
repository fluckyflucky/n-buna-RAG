package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/philippgille/chromem-go"
)

// TODO 对搜索出来的内容，调用API

type LLMRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"` // 设为 true 启用流式
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMStreamResponse Ollama 流式输出的单行结构
type LLMStreamResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
}

func newJSONPromptData(userInput string, searchResults []string) ([]byte, error) {
	// 数据库返回的内容
	contextText := strings.Join(searchResults, "\n---\n")

	// System Prompt
	systemPrompt := `你是一位专业的歌词创作助理，擅长模仿日本音乐人 n-buna (与 suis 组成乐队ヨルシカ) 的创作风格。
n-buna 的风格特点：
- 强烈的夏日感意象（入道云、蝉鸣、雨、花、车站、公交站）。
- 具有透明感的词汇和略显忧郁、遗憾的情绪。
- 善于观察生活中的细微之处，并赋予文学色彩。`

	userPrompt := fmt.Sprintf(
		"参考歌词片段：\n%s\n\n用户当前的灵感：[%s]\n请以此为基础生成三个段落的富有 n-buna 风格的歌词。",
		contextText, userInput)

	payload := LLMRequest{
		Model: "qwen3:8b",
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: true, // 开启流式传输
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}
func GenerateStream(ctx context.Context, userInput string, searchResults []string, outCh chan<- string) error {
	defer close(outCh) // 结束后关闭通道

	// 封装提示词
	jsonData, err := newJSONPromptData(userInput, searchResults)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// 5. 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API 错误: %d:%v", resp.StatusCode, resp.Body)
	}

	// 6. 逐行解析流式响应
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		var streamResp LLMStreamResponse
		if err = json.Unmarshal(line, &streamResp); err != nil {
			continue // 忽略解析失败的行
		}

		// 将新生成的片段发送到 Channel
		outCh <- streamResp.Message.Content

		if streamResp.Done {
			break
		}
	}

	return nil
}

// RunStreamDemo 演示流式调用
func RunStreamDemo(results []chromem.Result, input string) error {
	var resultStrings []string
	for _, result := range results {
		resultStrings = append(resultStrings, result.Content)
	}

	// 创建一个 Channel 来接收流式数据
	textCh := make(chan string)

	fmt.Println("AI 正在实时创作中...")

	// 开启协程执行生成
	go func() {
		err := GenerateStream(context.Background(), input, resultStrings, textCh)
		if err != nil {
			fmt.Printf("\n生成出错: %v\n", err)
		}
	}()

	// 在主线程中实时打印 Channel 里的内容
	for fragment := range textCh {
		fmt.Print(fragment) //用 Print 而不是 Println，实现打字机效果
	}
	fmt.Println("\n--- 创作完成 ---")
	return nil
}
