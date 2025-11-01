package munger

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	jutil "github.com/tychoish/jasper/util"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
	"gopkg.in/yaml.v3"
)

type BlogPost struct {
	Metadata *BlogMetadata
	Path     string
	Body     string
}

func (p *BlogPost) WriteTo(buf *bufio.Writer) error {
	return fnx.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString("---\n")) }).Join(
		fnx.MakeWorker(func() error {
			out, err := yaml.Marshal(p.Metadata)
			if err != nil {
				return err
			}
			return ft.IgnoreFirst(buf.Write(out))
		}),
		fnx.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString("---\n\n")) }),
		fnx.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString(p.Body)) }),
	).Wait()
}

type BlogMetadata struct {
	Title      string   `yaml:"title"`
	Tags       []string `yaml:"tags,omitempty"`
	Categories []string `yaml:"categories,omitempty"`
	Date       string   `yaml:"date"`
	Author     string   `yaml:"author"`
	Markup     string   `yaml:"markup"`
}

func CollectFiles(rootPath string) *fun.Stream[BlogPost] {
	rootPath = jutil.TryExpandHomedir(rootPath)
	grip.Infof("collecting blog files: %s", rootPath)
	return libfun.WalkDirIterator(rootPath, func(path string, dir fs.DirEntry) (_ *BlogPost, err error) {
		if !strings.HasSuffix(path, ".rst") {
			return nil, nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil, ers.Wrap(err, path)
		}
		defer util.DropErrorOnDefer(file.Close)

		rawBody, err := io.ReadAll(file)
		if err != nil {
			return nil, ers.Wrap(err, path)
		}
		grip.Build().Level(level.Debug).Pair("path", util.TryCollapsePwd(path)).Pair("op", "collect").Send()
		lines := strings.Split(string(rawBody), "\n")

		var interval []int
		for idx, ln := range lines {
			if strings.TrimSpace(ln) == "---" {
				interval = append(interval, idx)
				continue
			}
			if len(interval) == 2 {
				break
			}
			if idx == 0 && len(interval) == 0 {
				grip.Build().Level(level.Warning).Pair("path", util.TryCollapsePwd(path)).Pair("state", "no header").Send()
				return &BlogPost{Path: path, Body: string(rawBody)}, nil
			}
			if idx == len(lines)-1 && len(interval) == 1 {
				return nil, ers.Wrap(ers.Error("unterminated metadata"), path)
			}
		}

		output := BlogPost{
			Metadata: &BlogMetadata{},
			Path:     path,
			Body:     strings.Join(lines[interval[1]+1:], "\n"),
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[interval[0]:interval[1]+1], "\n")), output.Metadata); err != nil {
			return nil, ers.Wrap(err, path)
		}

		return &output, nil
	})
}

func ConvertSite(ctx context.Context, path string) error {
	return CollectFiles(path).BufferParallel(runtime.NumCPU()).Parallel(fnx.NewHandler(func(ctx context.Context, p BlogPost) error {
		var stdoutBuf bytes.Buffer
		var stderrBuf bytes.Buffer

		err := jasper.Context(ctx).
			CreateCommand(ctx).
			Append("pandoc --from=rst --to=commonmark_x").
			SetOutputWriter(jutil.NewLocalBuffer(&stdoutBuf)).
			SetErrorWriter(jutil.NewLocalBuffer(&stderrBuf)).
			SetInput(bytes.NewBuffer([]byte(p.Body))).
			Run(ctx)
		if err != nil {
			return ers.Wrap(ers.Wrap(err, p.Path), stderrBuf.String())
		}

		p.Body = stdoutBuf.String()
		p.Body = strings.ReplaceAll(p.Body, `\'`, "'")
		p.Body = strings.ReplaceAll(p.Body, `\"`, `"`)
		p.Body = strings.ReplaceAll(p.Body, `\...`, `...`)
		if p.Metadata != nil {
			p.Metadata.Markup = "markdown"
		}

		grip.Build().Level(level.Debug).Pair("path", util.TryCollapsePwd(p.Path)).Pair("op", "transform").Send()

		return nil
	}).Join(func(ctx context.Context, p BlogPost) (err error) {
		defer func() { grip.Error(err) }()

		file, err := os.Create(strings.Replace(p.Path, ".rst", ".md", 1))
		if err != nil {
			return ers.Wrap(err, p.Path)
		}

		defer func() { err = erc.Join(err, file.Close()) }()
		grip.Infof("writing: %s", file.Name())

		buf := bufio.NewWriter(file)

		grip.Build().Level(level.Info).Pair("path", util.TryCollapsePwd(file.Name())).Pair("op", "writing").Send()

		if err := p.WriteTo(buf); err != nil {
			return ers.Wrap(err, p.Path)
		}
		if err := buf.Flush(); err != nil {
			return ers.Wrap(err, p.Path)
		}
		if err := file.Close(); err != nil {
			return ers.Wrap(err, p.Path)
		}

		return nil
	}), fun.WorkerGroupConfWorkerPerCPU()).Run(ctx)
}
