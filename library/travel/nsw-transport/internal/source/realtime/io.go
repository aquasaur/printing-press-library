package realtime

import "os"

// writeFile writes b to path, creating parent directories as needed.
func writeFile(path string, b []byte) (int64, error) {
	if dir := dirOf(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return 0, err
	}
	return int64(len(b)), nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}
