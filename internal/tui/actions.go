package tui

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/MagnumGoYB/aitok/internal/query"
	tea "github.com/charmbracelet/bubbletea"
)

func copyOSC52(value string) tea.Cmd {
	return func() tea.Msg {
		encoded := base64.StdEncoding.EncodeToString([]byte(value))
		fmt.Printf("\033]52;c;%s\a", encoded)
		return nil
	}
}

var writeClipboard = systemWriteClipboard

func copyToClipboard(value string) tea.Cmd {
	return func() tea.Msg {
		if writeClipboard(value) == nil {
			return nil
		}
		return copyOSC52(value)()
	}
}

func systemWriteClipboard(value string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = stringsNewReader(value)
		return cmd.Run()
	case "linux":
		var lastErr error
		for _, name := range []string{"wl-copy", "xclip", "xsel"} {
			var cmd *exec.Cmd
			switch name {
			case "wl-copy":
				cmd = exec.Command(name)
			case "xclip":
				cmd = exec.Command(name, "-selection", "clipboard")
			case "xsel":
				cmd = exec.Command(name, "--clipboard", "--input")
			}
			if cmd == nil {
				continue
			}
			cmd.Stdin = stringsNewReader(value)
			if err := cmd.Run(); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return lastErr
	case "windows":
		cmd := exec.Command("cmd", "/c", "clip")
		cmd.Stdin = stringsNewReader(value)
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported clipboard platform %s", runtime.GOOS)
	}
}

func stringsNewReader(value string) *strings.Reader {
	return strings.NewReader(value)
}

func normalizePayloadSort(sortBy query.SortMetric) query.SortMetric {
	if sortBy == query.SortByCost {
		return query.SortByCost
	}
	return query.SortByTokens
}

func toggleSortMetric(sortBy query.SortMetric) query.SortMetric {
	if normalizePayloadSort(sortBy) == query.SortByCost {
		return query.SortByTokens
	}
	return query.SortByCost
}
