package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Analyze - точка входа в краулер
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: opts.Timeout,
		}
	}

	p, err := newPool(ctx, opts)
	if err != nil {
		return nil, err
	}
	p.start()
	p.wait()
	p.close()

	response := parseResult(p)

	indent := "  "
	if opts.IndentJSON > 0 {
		indent = strings.Repeat(" ", opts.IndentJSON)
	}

	return json.MarshalIndent(response, "", indent)
}
