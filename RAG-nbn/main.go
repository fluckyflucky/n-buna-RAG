package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// Chunk 切出的块，存储歌词的ID、标题、内容、行数和嵌入向量
type Chunk struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Lines     []string  `json:"lines"`
	Embedding []float32 `json:"embedding"`
}

type OllamaEmbedding struct {
	Embedding []float32 `json:"embedding"`
}

// CleanText 清除空行
func CleanText(text string) []string {
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		cleanLine := strings.TrimSpace(line)
		if cleanLine != "" {
			cleanLines = append(cleanLines, cleanLine)
		}
	}
	return cleanLines
}

// SlidingWindow 滑动窗口切块
func SlidingWindow(title string, lines []string, windowSize int, overlap int) []Chunk {
	var chunks []Chunk
	if windowSize < 3 {
		windowSize = 3
	}
	step := windowSize - overlap
	if step <= 0 {
		step = 1
	}
	count := 0
	for i := 0; i < len(lines); i += step {
		end := min(i+windowSize, len(lines)) // 窗口右边框

		// 提取当前窗口的行
		content := strings.Join(lines[i:end], " ")
		chunk := Chunk{
			ID:      count,
			Title:   title,
			Content: content,
			Lines:   lines[i:end],
		}
		chunks = append(chunks, chunk)
		count++

		if end == len(lines) {
			break
		}
	}
	return chunks
}

// fetchEmbedding 对输入的内容，获取向量
func fetchEmbedding(modelName string, prompt string) ([]float32, error) {
	url := "http://localhost:11434/api/embeddings"
	requestBody, _ := json.Marshal(map[string]string{
		"model":  modelName,
		"prompt": prompt,
	})
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed: %d", response.StatusCode)
	}

	var result OllamaEmbedding
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

// ProcessVector 处理文件转成向量
func ProcessVector(filepath string, title string) ([]Chunk, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	lines := CleanText(string(data)) // 清空行

	chunks := SlidingWindow(title, lines, 6, 2)
	for i, chunk := range chunks {
		embedding, err := fetchEmbedding("bge-m3", chunk.Content) // 对每一行分别获取向量
		if err != nil {
			return nil, err
		}
		chunks[i].Embedding = embedding
	}

	return chunks, nil
}

func main() {
	filename := "lyrics/雲と幽霊.txt"
	title := "雲と幽霊"
	chunks, err := ProcessVector(filename, title)
	if err != nil {
		panic(err)
	}
	if len(chunks) > 0 {
		for _, chunk := range chunks {
			fmt.Println("(", chunk.ID, ")", chunk.Content, "--", chunk.Embedding)
		}
	}
	sm, err := NewStorageManager("./vec_")
	if err != nil {
		panic(err)
	}
	err = sm.SaveChunks(chunks)
	if err != nil {
		panic(err)
	}
	prompt := "真的很不可思议呢"

	inputVec, err := fetchEmbedding("bge-m3", prompt)
	if err != nil {
		panic(err)
	}
	// 搜索
	println("输入文本：", prompt)
	println("输入向量：", inputVec)
	log.Println("搜索结果：")
	results, err := sm.Search(context.Background(), inputVec, 5, "")
	if err != nil {
		return
	}
	for _, res := range results {
		fmt.Printf("[相似度: %.4f] 内容: %s\n", res.Similarity, res.Content)
	}
	err = RunStreamDemo(results, prompt)
	if err != nil {
		return
	}
}
