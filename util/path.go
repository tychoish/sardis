package util

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
)

func GetHomeDir() string {
	userHome, err := homedir.Dir()
	if err != nil {
		// workaround for cygwin if we're on windows but couldn't get a homedir
		if runtime.GOOS == "windows" && len(os.Getenv("HOME")) > 0 {
			userHome = os.Getenv("HOME")
		}
	}

	return userHome
}

func GetHostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "UNKNOWN_HOSTNAME"
	}
	return name
}

func TryExpandHomeDirs(in []string) []string {
	out := make([]string, len(in))

	for idx := range in {
		str := in[idx]
		if strings.HasPrefix(str, "~") {
			expanded, err := homedir.Expand(str)
			if err != nil {
				expanded = str
			}
			str = expanded
		}

		out[idx] = str
	}

	return out
}

func CollapseHomeDir(in string) string {
	dir, err := homedir.Dir()
	if err != nil {
		return in
	}
	if !strings.Contains(in, dir) {
		return in
	}
	in = strings.Replace(in, dir, "~", 1)
	if strings.HasSuffix(in, "~") {
		in = fmt.Sprint(in, string(filepath.Separator))
	}
	return in
}
