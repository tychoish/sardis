package util

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

func getCommand(ctx context.Context, args []string, dir string, env map[string]string) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	switch len(args) {
	case 0:
		return nil, errors.New("args invalid")
	case 1:
		if strings.Contains(args[0], " \"'") {
			spl, err := shlex.Split(args[0])
			if err != nil {
				return nil, errors.Wrap(err, "problem splitting argstring")
			}
			return getCommand(ctx, spl, dir, env)
		}
		cmd = exec.CommandContext(ctx, args[0])
	default:
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}
	cmd.Dir = dir

	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	return cmd, nil
}

func getLogOutput(out []byte) string {
	return strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->")
}

func RunCommand(ctx context.Context, id string, pri level.Priority, args []string, dir string, env map[string]string) error {
	cmd, err := getCommand(ctx, args, dir, env)
	if err != nil {
		return errors.WithStack(err)
	}

	out, err := cmd.CombinedOutput()
	grip.Log(pri, message.Fields{
		"id":   id,
		"cmd":  strings.Join(args, " "),
		"err":  err != nil,
		"path": dir,
		"out":  getLogOutput(out),
	})

	return errors.Wrap(err, "problem with command")
}

func RunCommandGroupContinueOnError(ctx context.Context, id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) error {
	catcher := grip.NewBasicCatcher()
	for idx, args := range cmds {
		cmd, err := getCommand(ctx, args, dir, env)
		if err != nil {
			return errors.WithStack(err)
		}

		out, err := cmd.CombinedOutput()
		catcher.Add(err)

		grip.Log(pri, message.Fields{
			"id":   id,
			"cmd":  strings.Join(args, " "),
			"err":  err != nil,
			"idx":  idx,
			"len":  len(cmds),
			"path": dir,
			"out":  getLogOutput(out),
		})
	}

	return catcher.Resolve()
}

func RunCommandGroup(ctx context.Context, id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) error {
	for idx, args := range cmds {
		cmd, err := getCommand(ctx, args, dir, env)
		if err != nil {
			return errors.WithStack(err)
		}

		out, err := cmd.CombinedOutput()

		grip.Log(pri, message.Fields{
			"id":   id,
			"cmd":  strings.Join(args, " "),
			"err":  err != nil,
			"idx":  idx,
			"len":  len(cmds),
			"path": dir,
			"out":  getLogOutput(out),
		})

		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}
