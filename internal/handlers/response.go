package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/lucasmeller1/excel_api/internal/config"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func JsonError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(ErrorResponse{Error: message})
	if err != nil {
		log.Println("json encode:", err)
	}
}

func LookupSchemaByGUID(s string) (string, bool) {
	schema, ok := config.GUIDToSchema[s]
	return schema, ok
}

func LookupGUIDBySchema(s string) (string, bool) {
	guid, ok := config.SchemaToGUID[s]
	return guid, ok
}
