package main

import (
	"os"
	"bufio"
	"strings"
	srv "github.com/9ziggy9/9ziggy9.ws/server"
)

func LoadEnv(filename string) error {
	srv.Log(srv.INFO, "loading environment variables from %s ...", filename)
	defer srv.Log(srv.SUCCESS, "environmental variables loaded.")

	file, err := os.Open(filename); if err != nil { return err }
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || strings.HasPrefix(line, "#") { continue } // comments

		kvp := strings.SplitN(line, "=", 2)
		if len(kvp) != 2 { continue }

		k := strings.TrimSpace(kvp[0])
		v := strings.TrimSpace(kvp[1])
		os.Setenv(k, v)
	}
	return scanner.Err()
}

const ENV_FILE string = "./.env";

