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
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

type NamedItem struct {
	Name string `bson:"name" json:"name" yaml:"name"`
}

func (ni NamedItem) String() string { return ni.Name }

type ArchLinuxPackagesAUR struct {
	Name   string `bson:"name" json:"name" yaml:"name"`
	Update bool   `bson:"update" json:"update" yaml:"update"`
}

type ArchPackage struct {
	Name           string
	Version        string
	InDistRepos    string
	ShouldUpdate   bool
	NoDependencies bool
}

func (pkg *ArchLinuxPackagesAUR) FetchPackage() fnx.Worker {
	return func(context.Context) error { return nil }
}

type ArchLinux struct {
	BuildPath   string                 `bson:"build_path" json:"build_path" yaml:"build_path"`
	AurPackages []ArchLinuxPackagesAUR `bson:"aur_packages" json:"aur_packages" yaml:"aur_packages"`
	Packages    []NamedItem            `bson:"packages" json:"packages" yaml:"packages"`

	cache struct {
		collectedAt         time.Time
		versions            dt.Map[string, string]
		explicitlyInstalled dt.Set[string]
		inSyncDB            dt.Set[string]
		absPackages         dt.Set[string]
		notInSyncDB         dt.Set[string]
		dependencies        dt.Set[string]
	}
}

func (conf *ArchLinux) DoDiscovery(ctx context.Context) error {
	return fun.MAKE.WorkerPool(
		fun.VariadicStream(
			conf.collectVersions,
			conf.collectExplicityInstalled,
			conf.collectInSyncDB,
			fnx.Worker(conf.collectNotInSyncDB).Join(conf.collectCurrentUsersABS),
		)).
		Join(fnx.MakeOperation(func() { conf.cache.collectedAt = time.Now() }).Worker()).
		Run(ctx)
}

func parsePacmanQline(line string) (zero dt.Tuple[string, string], _ error) {
	var out dt.Tuple[string, string]

	n, err := fmt.Sscan(line, &out.One, &out.Two)
	if err != nil {
		return zero, erc.Join(fmt.Errorf("%q is not a valid package spec, %d part(s)", line, n), ers.ErrCurrentOpSkip)
	}
	fun.Invariant.Ok(n != 2 && err == nil, "failed to parse package string", line, "without error reported")
	return out, nil
}

func processPackages(cmd string, adder fn.Handler[string]) fnx.Worker {
	return func(ctx context.Context) error {
		return fun.Convert(fnx.MakeConverter(func(in dt.Tuple[string, string]) string { return in.One })).
			Stream(fun.Convert(fnx.MakeConverterErr(parsePacmanQline)).
				Stream(libfun.RunCommand(ctx, cmd))).
			ReadAll(fnx.FromHandler(adder)).Run(ctx)
	}
}

func (conf *ArchLinux) collectVersions(ctx context.Context) error {
	if conf.cache.versions == nil {
		conf.cache.versions = map[string]string{}
	}
	return fun.Convert(fnx.MakeConverterErr(parsePacmanQline)).
		Stream(libfun.RunCommand(ctx, "pacman --query")).
		ReadAll(fnx.FromHandler(conf.cache.versions.AddTuple)).Run(ctx)
}

func (conf *ArchLinux) collectExplicityInstalled(ctx context.Context) error {
	return processPackages("pacman --query --explicit", conf.cache.explicitlyInstalled.Add).Run(ctx)
}

func (conf *ArchLinux) collectInSyncDB(ctx context.Context) error {
	return processPackages("pacman --query --native", conf.cache.inSyncDB.Add).Run(ctx)
}

func (conf *ArchLinux) collectNotInSyncDB(ctx context.Context) error {
	return processPackages("pacman --query --foreign", conf.cache.notInSyncDB.Add).Run(ctx)
}

func (conf *ArchLinux) collectNoDependents(ctx context.Context) error {
	return processPackages("pacman --query --deps", conf.cache.dependencies.Add).Run(ctx)
}

func (conf *ArchLinux) collectCurrentUsersABS(ctx context.Context) error {
	absPath := filepath.Join(util.GetHomeDir(), "abs")
	return libfun.RunCommand(ctx, fmt.Sprintf("find %s -name %q", absPath, "PKGBUILD")).
		Transform(fnx.MakeConverter(func(path string) string {
			path = strings.Replace(path, absPath, "", 1)
			path = strings.Replace(path, "/PKGBUILD", "", 1)
			path = strings.Trim(path, " / ")
			if strings.ContainsAny(path, "/") {
				return ""
			}
			return path
		})).
		Filter(conf.cache.notInSyncDB.Check).
		ReadAll(fnx.FromHandler(conf.cache.absPackages.Add)).
		Run(ctx)
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
			ec.Push(fmt.Errorf("making %q: %w", conf.BuildPath, err))
		}
	} else if !stat.IsDir() {
		ec.Push(fmt.Errorf("arch build path '%s' is a file not a directory", conf.BuildPath))
	}

	for idx, pkg := range conf.AurPackages {
		if pkg.Name == "" {
			ec.Push(fmt.Errorf("aur package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			ec.Push(fmt.Errorf("aur package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}

	for idx, pkg := range conf.Packages {
		if pkg.Name == "" {
			ec.Push(fmt.Errorf("package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			ec.Push(fmt.Errorf("package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}
	return ec.Resolve()
}

func (conf *ArchLinux) FetchPackageFromAUR(name string, update bool) fnx.Worker {
	const opName = "arch-build-abs"

	hn := util.GetHostname()

	return func(ctx context.Context) error {
		if name == "" {
			return errors.New("aur package name is not specified")
		}

		startAt := time.Now()
		nonce := strings.ToLower(rand.Text())[:7]

		dir := filepath.Join(conf.BuildPath, name)
		repo := repo.GitRepository{
			Name:       name,
			Path:       dir,
			Remote:     fmt.Sprintf("https://aur.archlinux.org/%s.git", name),
			RemoteName: fmt.Sprint("aur.", name),
			Branch:     "master",
			Fetch:      true,
		}
		for _, pk := range conf.AurPackages {
			if pk.Name == name {
				return repo.UpdateJob().Run(ctx)
			}
		}

		ec := &erc.Collector{}

		return repo.FetchJob().
			PreHook(func(context.Context) {
				grip.Info(message.BuildPair().
					Pair("op", opName).
					Pair("state", "STARTED").
					Pair("pkg", name).
					Pair("class", "fetch").
					Pair("ID", nonce).
					Pair("host", hn))
			}).
			WithErrorHook(ec.Push).
			PostHook(func() {
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
			}).Run(ctx)
	}
}

func (conf *ArchLinux) BuildPackageInABS(name string) fnx.Worker {
	const opName = "arch-build-abs"
	return func(ctx context.Context) error {
		if name == "" {
			return errors.New("aur package name is not specified")
		}

		startAt := time.Now()
		nonce := strings.ToLower(rand.Text())[:7]

		dir := filepath.Join(conf.BuildPath, name)

		if err := conf.FetchPackageFromAUR(name, true).Run(ctx); err != nil {
			return err
		}

		pkgbuild := filepath.Join(dir, "PKGBUILD")
		if _, err := os.Stat(pkgbuild); os.IsNotExist(err) {
			return fmt.Errorf("could not resolve or update AUR package for %s", name)
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
			WithErrorHook(ec.Push).
			PostHook(func() {
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
					return
				}

				grip.Notice(msg)
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

func (conf *ArchLinux) InstallPackages() fnx.Worker {
	return func(ctx context.Context) error {
		return jasper.Context(ctx).
			CreateCommand(ctx).
			Priority(level.Info).
			Add(append([]string{"pacman", "--sync", "--refresh"}, conf.GetPackageNames()...)).
			SetOutputSender(level.Info, grip.Sender()).
			Run(ctx)
	}
}
