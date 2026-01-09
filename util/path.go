package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tychoish/fun/erc"
	jutil "github.com/tychoish/jasper/util"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func Apply[T any](fn func(T) T, in []T) []T {
	out := make([]T, len(in))

	for idx := range in {
		out[idx] = fn(in[idx])
	}

	return out
}

func TryExpandHomeDirs(in []string) []string { return Apply(TryExpandHomeDir, in) }
func GetHomeDir() string                     { return jutil.GetHomedir() }

func TryExpandHomeDir(in string) string {
	in = strings.TrimSpace(in)

	if len(in) == 0 || in[0] != '~' {
		return in
	}

	if len(in) > 1 && in[1] != '/' && in[1] != '\\' {
		// these are "~foo" or "~\" values which are ambiguous
		return in
	}

	return filepath.Join(GetHomeDir(), in[1:])
}

func TryCollapseHomeDir(in string) string {
	hd := jutil.GetHomedir()
	if strings.HasPrefix(in, hd) {
		return strings.Replace(in, hd, "~", 1)
	}
	return in
}

func TryCollapsePwd(in string) string {
	dir := erc.Must(filepath.Abs(jutil.TryExpandHomedir(in)))
	cwd := erc.Must(os.Getwd())

	if strings.HasPrefix(dir, cwd) {
		return strings.Replace(dir, cwd, ".", 1)
	}

	return in
}

func GetAlacrittySocketPath() (out string, err error) {
	if path, ok := os.LookupEnv("ALACRITTY_SOCKET"); ok {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	base := filepath.Join("/run/user", usr.Uid)
	socketPrefix := filepath.Join(base, "Alacritty-:")
	err = filepath.Walk(base,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}
			if !strings.HasPrefix(path, socketPrefix) {
				return nil
			}

			out = path
			return io.EOF
		})
	if out == "" || (err != nil && !errors.Is(err, io.EOF)) {
		return "", fmt.Errorf("no socket [%s] found: %w", out, err)
	}

	return out, nil
}

func GetSSHAgentPath() (out string, err error) {
	if path, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	if path := filepath.Join("/run/user", usr.Uid, "ssh-agent.socket"); jutil.FileExists(path) {
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
