package handlers

import (
	"encoding/json"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	db        *pgxpool.Pool
	AliasMap  map[string]string
	Blocklist map[string]bool
}

func New(db *pgxpool.Pool, aliasMapPath, blocklistPath string) *Handler {
	aliasMap := make(map[string]string)
	data, err := os.ReadFile(aliasMapPath)
	if err == nil {
		json.Unmarshal(data, &aliasMap)
	}

	blocklist := make(map[string]bool)
	blocklistData, err := os.ReadFile(blocklistPath)
	if err == nil {
		var list []string
		if json.Unmarshal(blocklistData, &list) == nil {
			for _, item := range list {
				canonical := item
				if val, ok := aliasMap[item]; ok && val != "" {
					canonical = val
				}
				blocklist[canonical] = true
			}
		}
	}

	return &Handler{
		db:        db,
		AliasMap:  aliasMap,
		Blocklist: blocklist,
	}
}
