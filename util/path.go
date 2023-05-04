package util

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

var (
	hostNameCache *adt.Once[string]
	homeDirCache  *adt.Once[string]
)

func init() {
	hostNameCache = &adt.Once[string]{}
	homeDirCache = &adt.Once[string]{}
}

func GetHomeDir() string {
	return homeDirCache.Do(func() string {
		userHome, err := homedir.Dir()
		if err != nil {
			// workaround for cygwin if we're on windows but couldn't get a homedir
			if runtime.GOOS == "windows" && len(os.Getenv("HOME")) > 0 {
				userHome = os.Getenv("HOME")
			}
		}

		return userHome
	})
}

func GetHostname() string {
	return hostNameCache.Do(func() string {
		name, err := os.Hostname()
		if err != nil {
			return "UNKNOWN_HOSTNAME"
		}
		return name
	})
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
	dir := GetHomeDir()

	if !strings.Contains(in, dir) {
		return in
	}

	in = strings.Replace(in, dir, "~", 1)
	if strings.HasSuffix(in, "~") {
		in = fmt.Sprint(in, string(filepath.Separator))
	}

	return in
}

func GetDirectoryContents(path string) (fun.Iterator[string], error) {
	dir, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return fun.Generator(func(ctx context.Context) (string, error) {
		dir, err := dir.ReadDir(1)
		if err != nil {
			return "", err
		}

		fun.Invariant(len(dir) == 1, "impossible return value from ReadDir")

		return filepath.Join(path, dir[0].Name()), nil
	}), nil
}

func GetSSHAgentPath() (out string, err error) {
	if path, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	if path := filepath.Join("/run/user", usr.Uid, "ssh-agent-socket"); FileExists(path) {
		return path, nil
	}

	err = filepath.Walk("/tmp", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(path, "/tmp/ssh-") {
			return nil
		}

		out = path
		return io.EOF
	})

	if out == "" || err == nil {
		return "", errors.New("could not find ssh socket")
	}
	return out, nil
}
