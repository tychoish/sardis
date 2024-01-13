package munger

import (
	"context"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/libfun"
	"gopkg.in/yaml.v3"
)

type BlogPost struct {
	Metadata *BlogMetadata
	Path     string
	Body     string
}
type BlogDate struct{ time.Time }

func (b *BlogDate) MarshalYAML() (any, error) {
	return yaml.Marshal(b.Time.Format("2006-01-02"))
}

func (b *BlogDate) UnmarshalYAML(node *yaml.Node) (any, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, ers.Join(ers.Error("parsing date field"), ers.ErrInvalidInput)
	}
	val, err := time.Parse("2006-01-02", node.Value)
	if err != nil {
		return nil, err
	}
	b.Time = val
	return val, err
}

type BlogMetadata struct {
	Title      string   `yaml:"author"`
	Tags       []string `yaml:"tags"`
	Categories []string `yaml:"categories"`
	Date       BlogDate `yaml:"date"`
	Author     string   `yaml:"author"`
	Markup     string   `yaml:"markup"`
}

func CollectFiles(rootPath string) *fun.Iterator[BlogPost] {
	return libfun.WalkDirIterator(rootPath, func(path string, dir fs.DirEntry) (_ *BlogPost, err error) {
		if !strings.HasSuffix(".rst") {
			return nil, nil
		}

		defer func() { err = ers.Wrap(err, path) }()
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

		var interval []int
		for idx, ln := range lines {
			if strings.HasPrefix("---", ln) {
				interval = append(interval, idx)
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
			Body:     string(rawBody),
		}
		if err := yaml.Unmarshal([]byte(strings.Join(lines[interval[0]:interval[1]], "\n")), output.Metadata); err != nil {
			return nil, err
		}

		return &output, nil
	})
}

func RstBlogPostToMarkdown(ctx context.Context, input *BlogPost) (*BlogPost, error) {
	out, err := libfun.RunCommandWithInput(ctx, "pandoc --from=rst --to=commonmark_x", strings.NewReader(input.Body)).Slice(ctx)
	if err != nil {
		return nil, err
	}

	output := input
	output.Body = strings.Join(out, "\n")
	if output.Metadata != nil {
		output.Metadata.Markup = "markdown"
	}
	return output, nil
}
