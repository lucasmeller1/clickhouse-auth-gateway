package clickhouse

import (
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"net/http"

	"github.com/lucasmeller1/excel_api/internal/handlers"
)

func (c *HTTPCSVClient) GetUserTables(w http.ResponseWriter, r *http.Request) {
	claims, ok := handlers.ClaimsFromContext(r.Context())
	if !ok {
		handlers.JsonError(w, http.StatusInternalServerError, "failed to parse authorization claims")
		return
	}

	authorizedSet := make(map[string]struct{})
	for _, s := range c.publicSchemas {
		authorizedSet[s] = struct{}{}
	}

	for _, guid := range claims.Groups {
		if schema, found := handlers.LookupSchemaByGUID(guid); found {
			authorizedSet[schema] = struct{}{}
		}
	}

	if len(authorizedSet) == 0 {
		handlers.JsonError(w, http.StatusForbidden, "no authorized databases found for your account")
		return
	}

	var schemas []string
	for s := range authorizedSet {
		schemas = append(schemas, fmt.Sprintf("'%s'", s))
	}
	inClause := strings.Join(schemas, ",")

	sql := fmt.Sprintf(`
		SELECT 
			database, 
			name, 
			concat(
				'http://%s/export?database=',
				database,
				'&table=',
				name
			) AS download_url
		FROM system.tables
		WHERE database IN (%s)
		ORDER BY
			database ASC,
			name ASC`,
		r.Host,
		inClause,
	)

	ctx := r.Context()
	resp, err := c.QueryCSV(ctx, sql)
	if err != nil {
		handlers.JsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")

	if resp.Header.Get("Content-Encoding") == "gzip" && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		io.Copy(w, resp.Body)
		return
	}

	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, _ := gzip.NewReader(resp.Body)
		defer gz.Close()
		io.Copy(w, gz)
		return
	}

	io.Copy(w, resp.Body)
}
