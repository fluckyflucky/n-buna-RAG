package main

import (
	"context"
	"fmt"

	"github.com/philippgille/chromem-go"
)

type StorageManager struct {
	DB         *chromem.DB
	Collection *chromem.Collection
	Path       string
}

// NewStorageManager 初始化或加载本地数据库
func NewStorageManager(dbPath string) (*StorageManager, error) {
	db, _ := chromem.NewPersistentDB(dbPath, false)

	// 已存在则获取，不存在则创建
	col, err := db.GetOrCreateCollection("n_buna_lyrics", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("无法获取集合: %v", err)
	}
	return &StorageManager{
		DB:         db,
		Collection: col,
		Path:       dbPath,
	}, nil
}
func (s *StorageManager) SaveChunks(chunks []Chunk) error {
	var documents []chromem.Document

	for _, chunk := range chunks {
		documents = append(documents, chromem.Document{
			ID:        fmt.Sprintf("%v_%d", chunk.Title, chunk.ID),
			Content:   chunk.Content,
			Embedding: chunk.Embedding,
			Metadata:  map[string]string{"title": chunk.Title},
		})
	}

	err := s.Collection.AddDocuments(context.Background(), documents, 1)
	if err != nil {
		return fmt.Errorf("添加文档失败: %v", err)
	}
	return nil
}

// Search 搜索最相似的歌词片段
// titleFilter: 如果不为空，则只搜索特定标题的歌词
func (s *StorageManager) Search(ctx context.Context, queryEmbedding []float32, topK int, titleFilter string) ([]chromem.Result, error) {
	var where map[string]string //初始nil

	if titleFilter != "" {
		where = map[string]string{"title": titleFilter}
	}

	// QueryEmbedding 会在本地索引中查找最接近的向量
	return s.Collection.QueryEmbedding(ctx, queryEmbedding, topK, where, nil)
}
