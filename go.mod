module github.com/tychoish/sardis

go 1.14

replace github.com/nutmegdevelopment/sumologic => github.com/tychoish/sumologic v0.0.0-20200521155714-c2840dd463d0

require (
	github.com/deciduosity/amboy v0.0.0-20200529182733-b32c6eeef7f5
	github.com/deciduosity/grip v0.0.0-20200529193719-caaa6d86281e
	github.com/deciduosity/jasper v0.0.0-20200525185637-a2512bf662c2
	github.com/deciduosity/utility v0.0.0-20200521233144-556c4888c866
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.4
	gopkg.in/yaml.v2 v2.3.0
)
