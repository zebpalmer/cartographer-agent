package common

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// PostReport sends a POST request to the given URL with the provided JSON data and token.
func PostReport(url string, jsonValue []byte, token string, useGzip bool) (map[string]interface{}, error) {
	var bodyBuffer io.Reader
	var req *http.Request
	var err error

	if useGzip {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		if _, err := w.Write(jsonValue); err != nil {
			slog.Error("Error compressing data",
				slog.String("error", err.Error()),
			)
			return nil, err
		}
		err := w.Close()
		if err != nil {
			return nil, err
		}
		bodyBuffer = &b
		req, err = http.NewRequest(http.MethodPost, url, bodyBuffer)
		req.Header.Set("Content-Encoding", "gzip")
	} else {
		bodyBuffer = bytes.NewBuffer(jsonValue)
		req, err = http.NewRequest(http.MethodPost, url, bodyBuffer)
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}

	if err != nil {
		slog.Error("Error creating HTTP request",
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	req.Header.Set("Authorization", token)

	client := http.Client{
		Timeout: 30 * time.Second,
	}
	slog.Debug("Posting report", slog.String("url", url))

	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("Error during HTTP request",
			slog.String("error", err.Error()),
		)
		return nil, err
	} else if resp.StatusCode != 202 {
		msg := fmt.Sprintf("HTTP Status code: %d", resp.StatusCode)
		slog.Warn("Unexpected HTTP status code", slog.Int("status_code", resp.StatusCode))
		return nil, errors.New(msg)
	} else {
		body, _ := io.ReadAll(resp.Body)
		data := map[string]interface{}{}
		err := json.Unmarshal(body, &data)
		if err != nil {
			slog.Error("Error unmarshalling JSON response",
				slog.String("error", err.Error()),
			)
			return nil, err
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				slog.Error("Error closing response body",
					slog.String("error", err.Error()),
				)
			}
		}(resp.Body)
		return data, nil
	}
}

// ToJSON converts a map to a JSON byte slice.
func ToJSON(data map[string]interface{}) ([]byte, error) {
	return json.Marshal(data)
}
