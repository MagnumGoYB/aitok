package sources

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func readJSONLines(ctx context.Context, path string, handle func(map[string]any) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 256*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var obj map[string]any
			if jsonErr := json.Unmarshal(bytes.TrimSpace(line), &obj); jsonErr == nil {
				if handleErr := handle(obj); handleErr != nil {
					return handleErr
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func intValue(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func objectValue(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
