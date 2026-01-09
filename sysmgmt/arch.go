package sysmgmt

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
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
		versions            stw.Map[string, string]
		explicitlyInstalled dt.Set[string]
		inSyncDB            dt.Set[string]
		absPackages         dt.Set[string]
		notInSyncDB         dt.Set[string]
		dependencies        dt.Set[string]
	}
}

func (conf *ArchLinux) Discovery() fnx.Worker { return conf.doDiscovery }
func (conf *ArchLinux) doDiscovery(ctx context.Context) error {
	return wpa.RunWithPool(
		irt.Args(
			conf.collectVersions,
			conf.collectExplicityInstalled,
			conf.collectInSyncDB,
			conf.collectDependents,
			fnx.Worker(conf.collectNotInSyncDB).Join(conf.collectCurrentUsersABS),
		)).
		Join(fnx.MakeOperation(func() { conf.cache.collectedAt = time.Now() }).Worker()).
		Run(ctx)
}

func (conf *ArchLinux) ResolvePackages(ctx context.Context) iter.Seq[ArchPackage] {
	ec := &erc.Collector{}
	hostname := util.GetHostname()

	// Ensure discovery has run
	if time.Since(conf.cache.collectedAt) > time.Hour {
		ec.Push(conf.Discovery().Run(ctx))
	}

	return func(yield func(ArchPackage) bool) {
		// First yield discovered packages
		for k, v := range conf.cache.versions {
			pkg := conf.populatePackage(ArchPackage{
				Name:    k,
				Version: v,
				Tags:    []string{"installed", hostname, "discovery"},
			})
			if !yield(pkg) {
				return
			}
		}

		// Then yield configured packages
		for _, ap := range conf.Packages {
			ap = conf.populatePackage(ap)
			ap.Tags = append(ap.Tags, "from-config")
			if ap.State.Installed {
				ap.Tags = append(ap.Tags, "installed")
			} else {
				ap.Tags = append(ap.Tags, "missing")
			}
			if !yield(ap) {
				return
			}
		}
	}
}

func (conf *ArchLinux) populatePackage(ap ArchPackage) ArchPackage {
	ap.State.InDistRepos = conf.cache.inSyncDB.Check(ap.Name)
	ap.State.InUsersABS = conf.cache.absPackages.Check(ap.Name)
	ap.State.AURpackageWithoutABS = !ap.State.InDistRepos && !ap.State.InUsersABS
	ap.State.ExplictlyInstalled = conf.cache.explicitlyInstalled.Check(ap.Name)
	ap.State.IsDependency = conf.cache.dependencies.Check(ap.Name)
	ap.State.Installed = conf.cache.versions.Check(ap.Name)

	if ap.State.InUsersABS && ap.PathABS == "" {
		ap.PathABS = filepath.Join(conf.BuildPath, ap.Name)
	}

	return ap
}

func parsePacmanQline(line string) (zero irt.KV[string, string], _ error) {
	var out irt.KV[string, string]

	n, err := fmt.Sscan(line, &out.Key, &out.Value)
	if err != nil {
		return zero, erc.Join(fmt.Errorf("%q is not a valid package spec, %d part(s)", line, n), ers.ErrCurrentOpSkip)
	}
	erc.InvariantOk(n != 2 && err == nil, "failed to parse package string", line, "without error reported")
	return out, nil
}

func processPackages(cmd string, adder func(string) error) fnx.Worker {
	return func(ctx context.Context) error {
		iter, err := libfun.RunCommand(ctx, cmd)
		if err != nil {
			return err
		}
		for line := range iter {
			kv, err := parsePacmanQline(line)
			if err != nil {
				if errors.Is(err, ers.ErrCurrentOpSkip) {
					continue
				}
				return err
			}
			if err := adder(kv.Key); err != nil {
				return err
			}
		}
		return nil
	}
}

func (conf *ArchLinux) collectVersions(ctx context.Context) error {
	if conf.cache.versions == nil {
		conf.cache.versions = map[string]string{}
	}
	iter, err := libfun.RunCommand(ctx, "pacman --query")
	if err != nil {
		return err
	}
	for line := range iter {
		kv, err := parsePacmanQline(line)
		if err != nil {
			if errors.Is(err, ers.ErrCurrentOpSkip) {
				continue
			}
			return err
		}
		conf.cache.versions.Store(kv.Key, kv.Value)
	}
	return nil
}

func (conf *ArchLinux) collectExplicityInstalled(ctx context.Context) error {
	return processPackages("pacman --query --explicit", func(s string) error {
		conf.cache.explicitlyInstalled.Add(s)
		return nil
	}).Run(ctx)
}

func (conf *ArchLinux) collectInSyncDB(ctx context.Context) error {
	return processPackages("pacman --query --native", func(s string) error {
		conf.cache.inSyncDB.Add(s)
		return nil
	}).Run(ctx)
}

func (conf *ArchLinux) collectNotInSyncDB(ctx context.Context) error {
	return processPackages("pacman --query --foreign", func(s string) error {
		conf.cache.notInSyncDB.Add(s)
		return nil
	}).Run(ctx)
}

func (conf *ArchLinux) collectDependents(ctx context.Context) error {
	return processPackages("pacman --query --deps", func(s string) error {
		conf.cache.dependencies.Add(s)
		return nil
	}).Run(ctx)
}

func (conf *ArchLinux) collectCurrentUsersABS(ctx context.Context) error {
	iter, err := libfun.RunCommand(ctx, fmt.Sprintf("find %s -name %q", conf.BuildPath, "PKGBUILD"))
	if err != nil {
		return err
	}
	for path := range iter {
		path = strings.Replace(path, conf.BuildPath, "", 1)
		path = strings.Replace(path, "/PKGBUILD", "", 1)
		path = strings.Trim(path, " / ")
		if strings.ContainsAny(path, "/") {
			continue
		}
		if conf.cache.notInSyncDB.Check(path) {
			conf.cache.absPackages.Add(path)
		}
	}
	return nil
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

func (conf *ArchLinux) RepoPackages() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, pkg := range conf.Packages {
			if pkg.State.InDistRepos {
				if !yield(pkg.Name) {
					return
				}
			}
		}
	}
}

func (conf *ArchLinux) InstallPackages() fnx.Worker {
	return func(ctx context.Context) error {
		pkgs := irt.Collect(conf.RepoPackages())
		args := append([]string{"pacman", "--sync", "--refresh"}, pkgs...)
		return jasper.Context(ctx).
			CreateCommand(ctx).
			Priority(level.Info).
			Add(args).
			SetOutputSender(level.Info, grip.Sender()).
			Run(ctx)
	}
}
