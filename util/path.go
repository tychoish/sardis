package util

import (
	"context"
	"errors"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tychoish/fun"
	jutil "github.com/tychoish/jasper/util"
)

func Apply[T any](fn func(T) T, in []T) []T {
	out := make([]T, len(in))

	for idx := range in {
		out[idx] = fn(in[idx])
	}

	return out
}

func TryExpandHomeDirs(in []string) []string { return Apply(jutil.TryExpandHomedir, in) }

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

func TryCollapseHomedir(in string) string {
	hd := jutil.GetHomedir()
	if strings.HasPrefix(in, hd) {
		return strings.Replace(in, hd, "~", 1)
	}
	return in
}

func GetSSHAgentPath() (out string, err error) {
	if path, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	if path := filepath.Join("/run/user", usr.Uid, "ssh-agent-socket"); jutil.FileExists(path) {
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
