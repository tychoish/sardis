module github.com/tychoish/sardis

go 1.14

replace github.com/nutmegdevelopment/sumologic => github.com/tychoish/sumologic v0.0.0-20200521155714-c2840dd463d0

require (
	github.com/aws/aws-sdk-go v1.31.4 // indirect
	github.com/deciduosity/amboy v0.0.0-20200522022153-94c42ab9205c
	github.com/deciduosity/grip v0.0.0-20200523175710-fd8be2e0aab0
	github.com/deciduosity/jasper v0.0.0-20200523230815-c72d4432c8d7
	github.com/frankban/quicktest v1.10.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.4
	gopkg.in/yaml.v2 v2.3.0
)
