//go:build tinygo

package sdk

import (
	"context"
	"fmt"

	fetch_ "marwan.io/wasm-fetch"
)

func fetch(ctx context.Context, url string) ([]byte, error) {
	resp, err := fetch_.Fetch(url, &fetch_.Opts{
		Method: fetch_.MethodGet,
		Signal: ctx,
	})
	if err != nil {
		return nil, err
	}
	if resp.Status >= 300 {
		return nil, fmt.Errorf("status code %d", resp.Status)
	}

	return resp.Body, nil
}
