package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

const envPrefix = "OLDTRALIA_"

type Env struct {
	values map[string]string
}

func LoadEnv(path string) (Env, error) {
	e := Env{values: map[string]string{}}
	f, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return e, err
	}
	if err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			key, value, _ := strings.Cut(line, "=")
			value = strings.TrimSpace(value)
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}
			e.values[strings.TrimSpace(key)] = value
		}
		_ = f.Close()
		if err := sc.Err(); err != nil {
			return e, err
		}
	}
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(key, envPrefix) {
			e.values[key] = value
		}
	}
	return e, nil
}

func (e Env) Get(key, def string) string {
	if v, ok := e.values[key]; ok {
		return v
	}
	return def
}

func (e Env) Int(key string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(e.values[key]))
	if err != nil {
		if f, ferr := strconv.ParseFloat(strings.TrimSpace(e.values[key]), 64); ferr == nil {
			return int(f)
		}
		return def
	}
	return v
}

func (e Env) Bool(key string, def bool) bool {
	switch strings.ToLower(e.values[key]) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}
