package clickhouse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	//"log"
	"net/http"
	"time"

	"github.com/lucasmeller1/excel_api/internal/config"
)

type HTTPCSVClient struct {
	baseURL string
	user    string
	pass    string
	client  *http.Client
}

func NewHTTPCSV(cfg config.ClickhouseConfig) *HTTPCSVClient {
	return &HTTPCSVClient{
		baseURL: cfg.Hostname,
		user:    cfg.User,
		pass:    cfg.Password,
		client: &http.Client{
			Timeout: time.Second * time.Duration(cfg.ClientTimeout),
			Transport: &http.Transport{
				DisableCompression: true,
			},
		},
	}
}

func (c *HTTPCSVClient) StreamCSV(
	ctx context.Context,
	sql string,
	out io.Writer,
) error {

	query := sql + " FORMAT CSVWithNames"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"?enable_http_compression=1",
		bytes.NewBufferString(query),
	)
	if err != nil {
		return err
	}

	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clickhouse error: %s", body)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *HTTPCSVClient) ExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	database := r.URL.Query().Get("database")
	table := r.URL.Query().Get("table")

	if database == "" || table == "" {
		http.Error(w, "missing database or table", 400)
		return
	}

	sql := fmt.Sprintf("SELECT * FROM %s.%s", database, table)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="data.csv"`)
	w.Header().Set("Content-Encoding", "gzip")

	err := c.StreamCSV(ctx, sql, w)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
