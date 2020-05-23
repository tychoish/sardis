module github.com/tychoish/sardis

go 1.14

replace github.com/nutmegdevelopment/sumologic => github.com/tychoish/sumologic v0.0.0-20200521155714-c2840dd463d0

require (
	github.com/deciduosity/amboy v0.0.0-20200522022153-94c42ab9205c
	github.com/deciduosity/grip v0.0.0-20200523031935-2e93f9b35e28
	github.com/deciduosity/jasper v0.0.0-20200522151905-d73a917c1259
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.4
	gopkg.in/yaml.v2 v2.3.0
)
