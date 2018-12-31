package util

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
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

func getRemoteCommand(ctx context.Context, host string, args []string, dir string) (*exec.Cmd, error) {
	var remoteCmd string

	if dir != "" {
		remoteCmd = fmt.Sprintf("cd %s && ", dir)
	}

	switch len(args) {
	case 0:
		return nil, errors.New("args invalid")
	case 1:
		remoteCmd += args[0]
	default:
		remoteCmd += strings.Join(args, " ")
	}

	return exec.CommandContext(ctx, "ssh", []string{host, remoteCmd}...), nil
}

func getLogOutput(out []byte) string {
	return strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->")
}

type Command struct {
	cmds     [][]string
	dir      string
	host     string
	env      map[string]string
	priority level.Priority
	id       string

	continueOnError bool
	stdOut          io.Writer
	stdErr          io.Writer
	closers         []func() error
}

func NewCommand() *Command                                  { return &Command{} }
func (c *Command) String() string                           { return fmt.Sprintf("id='%s', cmd='%s'", c.id, c.getCmd()) }
func (c *Command) Add(args []string) *Command               { c.cmds = append(c.cmds, args); return c }
func (c *Command) Extend(cmds [][]string) *Command          { c.cmds = append(c.cmds, cmds...); return c }
func (c *Command) Directory(d string) *Command              { c.dir = d; return c }
func (c *Command) Host(h string) *Command                   { c.host = h; return c }
func (c *Command) Priority(l level.Priority) *Command       { c.priority = l; return c }
func (c *Command) ID(id string) *Command                    { c.id = id; return c }
func (c *Command) SetContinue(ignore bool) *Command         { c.continueOnError = ignore; return c }
func (c *Command) Environment(e map[string]string) *Command { c.env = e; return c }
func (c *Command) AddEnv(k, v string) *Command              { c.setupEnv(); c.env[k] = v; return c }

func (c *Command) setupEnv() {
	if c.env == nil {
		c.env = map[string]string{}
	}
}

func (c *Command) Run(ctx context.Context) (err error) {
	c.finalizeWriters()
	catcher := grip.NewBasicCatcher()
	defer func() {
		catcher.Add(c.Close())
		err = catcher.Resolve()
	}()

	var cmds []*exec.Cmd
	cmds, err = c.getExecCmds(ctx)
	if err != nil {
		catcher.Add(err)
		return
	}

	for idx, cmd := range cmds {
		if err = ctx.Err(); err != nil {
			catcher.Add(errors.Wrap(err, "operation canceled"))
			return
		}

		err = c.exec(cmd, idx)
		catcher.Add(err)
		if c.continueOnError {
			continue
		} else if err != nil {
			return
		}
	}

	return
}

func (c *Command) Close() error {
	catcher := grip.NewBasicCatcher()
	for _, closer := range c.closers {
		catcher.Add(closer())
	}

	return catcher.Resolve()
}

func (c *Command) SetErrorSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriterSender(s, l)
	c.closers = append(c.closers, writer.Close)
	c.stdErr = writer
	return c
}

func (c *Command) SetOutputSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriterSender(s, l)
	c.closers = append(c.closers, writer.Close)
	c.stdOut = writer
	return c
}

func (c *Command) SetCombinedSender(l level.Priority, s send.Sender) *Command {
	writer := send.MakeWriterSender(s, l)
	c.closers = append(c.closers, writer.Close)
	c.stdOut = writer
	return c
}

func (c *Command) SetErrorWriter(writer io.WriteCloser) *Command {
	c.closers = append(c.closers, writer.Close)
	c.stdErr = writer
	return c
}

func (c *Command) SetOutputWriter(writer io.WriteCloser) *Command {
	c.closers = append(c.closers, writer.Close)
	c.stdOut = writer
	return c
}

func (c *Command) SetCombinedWriter(writer io.WriteCloser) *Command {
	c.closers = append(c.closers, writer.Close)
	c.stdOut = writer
	return c
}

func (c *Command) finalizeWriters() {
	if c.stdErr == nil && c.stdErr == nil {
		return
	}

	if c.stdErr != nil && c.stdOut == nil {
		c.stdOut = c.stdErr
	}

	if c.stdOut != nil && c.stdErr == nil {
		c.stdErr = c.stdOut
	}
}

func (c *Command) getEnv() []string {
	out := []string{}
	for k, v := range c.env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func (c *Command) getCmd() string {
	env := strings.Join(c.getEnv(), " ")
	out := []string{}
	for _, cmd := range c.cmds {
		out = append(out, fmt.Sprintf("%s '%s';\n", env, strings.Join(cmd, " ")))
	}
	return strings.Join(out, "")
}

func (c *Command) getExecCmds(ctx context.Context) ([]*exec.Cmd, error) {
	out := []*exec.Cmd{}
	// env := c.getEnv()
	catcher := grip.NewBasicCatcher()
	if c.host != "" {
		for _, args := range c.cmds {
			cmd, err := getRemoteCommand(ctx, c.host, args, c.dir)
			if err != nil {
				catcher.Add(err)
				continue
			}

			out = append(out, cmd)
		}
	} else {
		for _, args := range c.cmds {
			cmd, err := getCommand(ctx, args, c.dir, c.env)
			if err != nil {
				catcher.Add(err)
				continue
			}

			out = append(out, cmd)
		}
	}
	if catcher.HasErrors() {
		return nil, catcher.Resolve()
	}

	return out, nil
}

func (c *Command) exec(cmd *exec.Cmd, idx int) error {
	msg := message.Fields{
		"id":  c.id,
		"cmd": strings.Join(cmd.Args, " "),
		"idx": idx,
		"len": len(c.cmds),
	}

	var err error
	if c.stdOut == nil {
		var out []byte
		out, err = cmd.CombinedOutput()
		msg["out"] = getLogOutput(out)
		msg["err"] = err != nil
	} else {
		cmd.Stderr = c.stdErr
		cmd.Stdout = c.stdOut

		err = errors.Wrapf(cmd.Start(), "problem starting command")
		if err == nil {
			err = cmd.Wait()
		}
		msg["err"] = err != nil
	}
	grip.Log(c.priority, msg)
	return err
}

func RunCommand(ctx context.Context, id string, pri level.Priority, args []string, dir string, env map[string]string) error {
	return NewCommand().ID(id).Priority(pri).Add(args).Directory(dir).Environment(env).Run(ctx)
}

func RunRemoteCommand(ctx context.Context, id string, pri level.Priority, host string, args []string, dir string) error {
	return NewCommand().ID(id).Priority(pri).Host(host).Add(args).Directory(dir).Run(ctx)
}

func RunCommandGroupContinueOnError(ctx context.Context, id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) error {
	return NewCommand().ID(id).Priority(pri).Extend(cmds).Directory(dir).Environment(env).SetContinue(true).Run(ctx)
}

func RunRemoteCommandGroupContinueOnError(ctx context.Context, id string, pri level.Priority, host string, cmds [][]string, dir string) error {
	return NewCommand().ID(id).Priority(pri).Host(host).Extend(cmds).Directory(dir).SetContinue(true).Run(ctx)
}

func RunCommandGroup(ctx context.Context, id string, pri level.Priority, cmds [][]string, dir string, env map[string]string) error {
	return NewCommand().ID(id).Priority(pri).Extend(cmds).Directory(dir).Environment(env).Run(ctx)
}

func RunRemoteCommandGroup(ctx context.Context, id string, pri level.Priority, host string, cmds [][]string, dir string) error {
	return NewCommand().ID(id).Priority(pri).Host(host).Extend(cmds).Directory(dir).Run(ctx)
}
