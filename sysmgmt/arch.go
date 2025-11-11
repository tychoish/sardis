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

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

type ArchPackage struct {
	Name         string `bson:"name" json:"name" yaml:"name"`
	Version      string `bson:"version,omitempty" json:"version,omitempty" yaml:"version,omitempty"`
	ShouldUpdate bool   `bson:"update" json:"update" yaml:"update"`
	AUR          bool   `bson:"aur" json:"aur" yaml:"aur"`
	PathABS      string `bson:"abs_path,omitempty" json:"abs_path,omitempty" yaml:"abs_path,omitempty"`

	Tags []string `bson:"tags,omitempty" json:"tags,omitempty" yaml:"tags,omitempty"`

	State struct {
		InDistRepos          bool `bson:"from_arch_repo,omitempty" json:"from_arch_repo,omitempty" yaml:"from_arch_repo,omitempty"`
		InUsersABS           bool `bson:"abs,omitempty" json:"abs,omitempty" yaml:"abs,omitempty"`
		AURpackageWithoutABS bool `bson:"aur_without_build_directory,omitempty" json:"aur_without_build_directory,omitempty" yaml:"aur_without_build_directory,omitempty"`
		ExplictlyInstalled   bool `bson:"explictly_installed,omitempty" json:"explictly_installed,omitempty" yaml:"explictly_installed,omitempty"`
		IsDependency         bool `bson:"dependency,omitempty" json:"dependency,omitempty" yaml:"dependency,omitempty"`
		Installed            bool `bson:"-" json:"-" yaml:"-"`
	} `bson:"status,omitempty" json:"status,omitempty" yaml:"status,omitempty"`
}

type ArchLinux struct {
	BuildPath string        `bson:"build_path" json:"build_path" yaml:"build_path"`
	Packages  []ArchPackage `bson:"packages" json:"packages" yaml:"packages"`

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

func (conf *ArchLinux) Discovery() fnx.Worker { return conf.doDiscovery }
func (conf *ArchLinux) doDiscovery(ctx context.Context) error {
	return fun.MAKE.WorkerPool(
		fun.VariadicStream(
			conf.collectVersions,
			conf.collectExplicityInstalled,
			conf.collectInSyncDB,
			conf.collectDependents,
			fnx.Worker(conf.collectNotInSyncDB).Join(conf.collectCurrentUsersABS),
		)).
		Join(fnx.MakeOperation(func() { conf.cache.collectedAt = time.Now() }).Worker()).
		Run(ctx)
}

func (conf *ArchLinux) ResolvePackages(ctx context.Context) *fun.Stream[ArchPackage] {
	ec := &erc.Collector{}
	hostname := util.GetHostname()
	return fun.Convert(fnx.MakeConverter(func(pkg dt.Pair[string, string]) ArchPackage {
		return conf.populatePackage(ArchPackage{
			Name:    pkg.Key,
			Version: pkg.Value,
			Tags:    []string{"installed", hostname, "discovery"},
		})
	})).Parallel(
		fun.MakeStream(fnx.NewFuture(conf.cache.versions.Stream().Read).
			PreHook(conf.
				Discovery().
				When(func() bool {
					return time.Since(conf.cache.collectedAt) > time.Hour
				}).Once().Operation(ec.Push)),
		),
	).Join(
		fun.SliceStream(conf.Packages).
			Transform(fnx.MakeConverter(
				func(ap ArchPackage) ArchPackage {
					ap = conf.populatePackage(ap)
					ap.Tags = append(ap.Tags, "from-config")
					if ap.State.Installed {
						ap.Tags = append(ap.Tags, "installed")
					} else {
						ap.Tags = append(ap.Tags, "missing")
					}
					return ap
				}),
			),
	).BufferParallel(2)
}

func (conf *ArchLinux) populatePackage(ap ArchPackage) ArchPackage {
	ap.State.InDistRepos = conf.cache.inSyncDB.Check(ap.Name)
	ap.State.InUsersABS = conf.cache.absPackages.Check(ap.Name)
	ap.State.AURpackageWithoutABS = ft.Not(ap.State.InDistRepos) && ft.Not(ap.State.InUsersABS)
	ap.State.ExplictlyInstalled = conf.cache.explicitlyInstalled.Check(ap.Name)
	ap.State.IsDependency = conf.cache.dependencies.Check(ap.Name)
	ap.State.Installed = conf.cache.versions.Check(ap.Name)

	if ap.State.InUsersABS && ap.PathABS == "" {
		ap.PathABS = filepath.Join(conf.BuildPath, ap.Name)
	}

	return ap
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

func (conf *ArchLinux) collectDependents(ctx context.Context) error {
	return processPackages("pacman --query --deps", conf.cache.dependencies.Add).Run(ctx)
}

func (conf *ArchLinux) collectCurrentUsersABS(ctx context.Context) error {
	return libfun.RunCommand(ctx, fmt.Sprintf("find %s -name %q", conf.BuildPath, "PKGBUILD")).
		Transform(fnx.MakeConverter(func(path string) string {
			path = strings.Replace(path, conf.BuildPath, "", 1)
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
		conf.BuildPath = util.TryExpandHomeDir(conf.BuildPath)
	}

	ec := &erc.Collector{}
	if stat, err := os.Stat(conf.BuildPath); os.IsNotExist(err) {
		if err := os.MkdirAll(conf.BuildPath, 0o755); err != nil {
			ec.Push(fmt.Errorf("making %q: %w", conf.BuildPath, err))
		}
	} else if !stat.IsDir() {
		ec.Push(fmt.Errorf("arch build path '%s' is a file not a directory", conf.BuildPath))
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

func (conf *ArchLinux) RepoPackages() *fun.Stream[string] {
	return fun.Convert(fnx.MakeConverter(func(pkg ArchPackage) string {
		return pkg.Name
	})).Stream(fun.SliceStream(conf.Packages).
		Filter(func(pkg ArchPackage) bool {
			return pkg.State.InDistRepos
		}))
}

func (conf *ArchLinux) InstallPackages() fnx.Worker {
	return func(ctx context.Context) error {
		return jasper.Context(ctx).
			CreateCommand(ctx).
			Priority(level.Info).
			Add(append([]string{"pacman", "--sync", "--refresh"}, ft.IgnoreSecond(conf.RepoPackages().Slice(ctx))...)).
			SetOutputSender(level.Info, grip.Sender()).
			Run(ctx)
	}
}
