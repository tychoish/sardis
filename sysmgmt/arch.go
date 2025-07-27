package sysmgmt

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

type ArchLinux struct {
	BuildPath   string                 `bson:"build_path" json:"build_path" yaml:"build_path"`
	AurPackages []ArchLinuxPackagesAUR `bson:"aur_packages" json:"aur_packages" yaml:"aur_packages"`
	Packages    []NamedItem            `bson:"packages" json:"packages" yaml:"packages"`
}

type NamedItem struct {
	Name string `bson:"name" json:"name" yaml:"name"`
}

func (ni NamedItem) String() string { return ni.Name }

type ArchLinuxPackagesAUR struct {
	Name   string `bson:"name" json:"name" yaml:"name"`
	Update bool   `bson:"update" json:"update" yaml:"update"`
}

func (pkg *ArchLinuxPackagesAUR) FetchPackage() fun.Worker {
	return func(context.Context) error { return nil }
}

func (conf *ArchLinux) Validate() error {
	if _, err := os.Stat("/etc/arch-release"); os.IsNotExist(err) {
		return nil
	}

	if conf.BuildPath == "" {
		conf.BuildPath = filepath.Join(util.GetHomeDir(), "abs")
	} else {
		var err error

		conf.BuildPath, err = homedir.Expand(conf.BuildPath)
		if err != nil {
			return err
		}
	}

	ec := &erc.Collector{}
	if stat, err := os.Stat(conf.BuildPath); os.IsNotExist(err) {
		if err := os.MkdirAll(conf.BuildPath, 0o755); err != nil {
			ec.Add(fmt.Errorf("making %q: %w", conf.BuildPath, err))
		}
	} else if !stat.IsDir() {
		ec.Add(fmt.Errorf("arch build path '%s' is a file not a directory", conf.BuildPath))
	}

	for idx, pkg := range conf.AurPackages {
		if pkg.Name == "" {
			ec.Add(fmt.Errorf("aur package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			ec.Add(fmt.Errorf("aur package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}

	for idx, pkg := range conf.Packages {
		if pkg.Name == "" {
			ec.Add(fmt.Errorf("package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			ec.Add(fmt.Errorf("package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}
	return ec.Resolve()
}

func (conf *ArchLinux) FetchPackageFromAUR(name string, update bool) fun.Worker {
	const opName = "arch-build-abs"

	hn := util.GetHostname()

	return func(ctx context.Context) error {
		startAt := time.Now()
		nonce := strings.ToLower(rand.Text())[:7]
		if name == "" {
			for _, pk := range conf.AurPackages {
				if pk.Name == name {
					return pk.FetchPackage().Run(ctx)
				}
			}
			return errors.New("aur package name is not specified")
		}

		dir := filepath.Join(conf.BuildPath, name)

		repo := repo.GitRepository{
			Name:       name,
			Path:       dir,
			Remote:     fmt.Sprintf("https://aur.archlinux.org/%s.git", name),
			RemoteName: fmt.Sprint("aur.", name),
			Branch:     "master",
			Fetch:      true,
		}
		ec := &erc.Collector{}

		job := repo.FetchJob().
			PreHook(func(context.Context) {
				grip.Info(message.BuildPair().
					Pair("op", opName).
					Pair("state", "STARTED").
					Pair("pkg", name).
					Pair("class", "fetch").
					Pair("ID", nonce).
					Pair("host", hn))
			}).
			Operation(ec.Push).
			WithErrorHook(func() error {
				err := ec.Resolve()

				grip.Notice(message.BuildPair().
					Pair("op", opName).
					Pair("pkg", name).
					Pair("stage", "fetch").
					Pair("err", err != nil).
					Pair("state", "COMPLETED").
					Pair("dur", time.Since(startAt)).
					Pair("ID", nonce).
					Pair("host", hn))

				return err
			})

		if err := job.Run(ctx); err != nil {
			return err
		}
		return nil
	}
}

func (conf *ArchLinux) BuildPackageInABS(name string) fun.Worker {
	const opName = "arch-build-abs"
	return func(ctx context.Context) error {
		startAt := time.Now()
		nonce := strings.ToLower(rand.Text())[:7]

		if name == "" {
			for _, pk := range conf.AurPackages {
				if pk.Name == name {
					return pk.FetchPackage().Run(ctx)
				}
			}
			return errors.New("aur package name is not specified")
		}

		dir := filepath.Join(conf.BuildPath, name)
		pkgbuild := filepath.Join(dir, "PKGBUILD")

		if _, err := os.Stat(pkgbuild); os.IsNotExist(err) {
			if err := conf.FetchPackageFromAUR(name, true).Run(ctx); err != nil {
				return err
			}
		}

		hn := util.GetHostname()

		jobID := fmt.Sprintf("OP(%s).PKG(%s).HOST(%s).ID(%s)", opName, name, hn, nonce)

		proclog, buf := subexec.NewOutputBuf(fmt.Sprint(jobID, ".", nonce))
		proclog.Infoln("----------------", jobID, "--------------->")

		args := []string{"makepkg", "--syncdeps", "--force", "--install", "--noconfirm"}

		ec := &erc.Collector{}

		cmd := jasper.Context(ctx).CreateCommand(ctx).
			ID(jobID).
			Priority(level.Info).
			Directory(dir).
			AppendArgs(args...).
			SetOutputSender(level.Info, buf).
			SetErrorSender(level.Error, buf).
			Worker().
			PreHook(func(context.Context) {
				grip.Info(message.BuildPair().
					Pair("op", opName).
					Pair("state", "STARTED").
					Pair("stage", "build").
					Pair("pkg", name).
					Pair("host", hn))
			}).
			Operation(ec.Push).
			WithErrorHook(func() error {
				err := ec.Resolve()

				msg := message.BuildPair().
					Pair("op", opName).
					Pair("state", "STARTED").
					Pair("stage", "build").
					Pair("pkg", name).
					Pair("err", err != nil).
					Pair("dur", time.Since(startAt)).
					Pair("host", hn)

				if err != nil {
					proclog.Infoln("<---------------", jobID, "----------------")
					grip.Error(msg)
					grip.Info(buf.String())
					return err
				}

				grip.Notice(msg)
				return nil
			}).PostHook(func() { _ = buf.Close() })

		return cmd.Run(ctx)
	}
}

func (conf *ArchLinux) GetPackageNames() []string {
	pkgs := make([]string, 0, len(conf.Packages))
	for _, pkg := range conf.Packages {
		pkgs = append(pkgs, pkg.Name)
	}

	return pkgs
}

func (conf *ArchLinux) InstallPackages() fun.Worker {
	return func(ctx context.Context) error {
		return jasper.Context(ctx).
			CreateCommand(ctx).
			Priority(level.Info).
			Add(append([]string{"pacman", "--sync", "--refresh"}, conf.GetPackageNames()...)).
			SetOutputSender(level.Info, grip.Sender()).
			Run(ctx)
	}
}
