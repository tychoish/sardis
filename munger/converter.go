package munger

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/libfun"
	"gopkg.in/yaml.v3"
)

type BlogPost struct {
	Metadata *BlogMetadata
	Path     string
	Body     string
}

func (p *BlogPost) WriteTo(buf *bufio.Writer) error {
	return fun.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString("---\n")) }).Join(
		fun.MakeWorker(func() error {
			out, err := yaml.Marshal(p.Metadata)
			if err != nil {
				return err
			}
			return ft.IgnoreFirst(buf.Write(out))
		}),
		fun.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString("---\n\n")) }),
		fun.MakeWorker(func() error { return ft.IgnoreFirst(buf.WriteString(p.Body)) }),
	).Wait()
}

type BlogMetadata struct {
	Title      string   `yaml:"title"`
	Tags       []string `yaml:"tags"`
	Categories []string `yaml:"categories"`
	Date       string   `yaml:"date"`
	Author     string   `yaml:"author"`
	Markup     string   `yaml:"markup"`
}

func CollectFiles(rootPath string) *fun.Iterator[BlogPost] {
	grip.Infof("collecting blog files: %s", rootPath)
	return libfun.WalkDirIterator(rootPath, func(path string, dir fs.DirEntry) (_ *BlogPost, err error) {
		if !strings.HasSuffix(path, ".rst") {
			return nil, nil
		}
		grip.Infof("trying to read: %s", path)

		defer func() {
			err = ers.Wrap(err, path)
			grip.Error(err)
		}()

		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		rawBody, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(rawBody), "\n")
		grip.Infof("collected: %s", path)

		var interval []int
		for idx, ln := range lines {
			if strings.HasPrefix(ln, "---") {
				interval = append(interval, idx)
				continue
			}
			if len(interval) == 2 {
				break
			}
			if idx == 0 && len(interval) == 0 {
				return &BlogPost{Path: path, Body: string(rawBody)}, nil
			}
			if idx == len(lines)-1 && len(interval) == 1 {
				return nil, ers.Error("unterminated metadata")
			}
		}
		output := BlogPost{
			Metadata: &BlogMetadata{},
			Path:     path,
			Body:     string(rawBody[interval[1]:]),
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[interval[0]:interval[1]], "\n")), output.Metadata); err != nil {
			return nil, err
		}
		grip.Infof("produced: %s", path)

		return &output, nil
	})
}

func ConvertSite(ctx context.Context, path string) error {
	return CollectFiles(path).Transform(func(ctx context.Context, input BlogPost) (BlogPost, error) {
		out, err := libfun.RunCommandWithInput(ctx, "pandoc --from=rst --to=commonmark_x", strings.NewReader(input.Body)).Slice(ctx)
		if err != nil {
			grip.Error(err)
			return BlogPost{}, err
		}

		output := input
		output.Body = strings.Join(out, "\n")
		if output.Metadata != nil {
			output.Metadata.Markup = "markdown"
		}
		return output, nil
	}).ProcessParallel(func(ctx context.Context, p BlogPost) (err error) {
		defer func() { grip.Error(err) }()
		file, err := os.Create(strings.Replace(p.Path, ".rst", ".md", 1))
		if err != nil {
			return err
		}
		defer func() { err = ers.Join(err, file.Close()) }()
		grip.Infof("writing: %s", file.Name())
		buf := bufio.NewWriter(file)
		if err := p.WriteTo(buf); err != nil {
			return err
		}

		return buf.Flush()
	}).Run(ctx)
}
