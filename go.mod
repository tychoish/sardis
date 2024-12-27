module github.com/tychoish/sardis

go 1.23

toolchain go1.23.2

replace github.com/tychoish/libfun => ../libfun

replace github.com/tychoish/jasper => ../jasper

replace github.com/tychoish/fun => ../fun

require (
	github.com/Baozisoftware/qrcode-terminal-go v0.0.0-20170407111555-c0650d8dff0f
	github.com/cbrgm/githubevents v1.8.0
	github.com/cheynewallace/tabby v1.1.1
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/go-git/go-git/v5 v5.2.0
	github.com/google/go-github/v50 v50.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/nwidger/jsoncolor v0.3.2
	github.com/tychoish/birch v0.2.3-0.20230815163402-467fbef7acab
	github.com/tychoish/cmdr v0.3.5-0.20240114233549-d04dd21b99b4
	github.com/tychoish/fun v0.10.9
	github.com/tychoish/godmenu v0.1.2
	github.com/tychoish/grip v0.3.8-0.20240114232258-7eae5cf3a031
	github.com/tychoish/grip/x/desktop v0.0.0-20240114232258-7eae5cf3a031
	github.com/tychoish/grip/x/jira v0.0.0-20240114232258-7eae5cf3a031
	github.com/tychoish/grip/x/system v0.0.0-20240114232258-7eae5cf3a031
	github.com/tychoish/grip/x/telegram v0.0.0-20240114232258-7eae5cf3a031
	github.com/tychoish/grip/x/twitter v0.0.0-20230815172847-a642e6ca055e
	github.com/tychoish/grip/x/xmpp v0.0.0-20240114232258-7eae5cf3a031
	github.com/tychoish/jasper v0.1.2-0.20240114233130-6f760329794b
	github.com/tychoish/jasper/x/cli v0.0.0-20230825020900-7d32edd66d81
	github.com/tychoish/jasper/x/track v0.0.0-20230509174929-2fe8b231212c
	github.com/tychoish/libfun v0.0.0-00010101000000-000000000000
	github.com/urfave/cli/v2 v2.25.3
	go.mongodb.org/mongo-driver v1.11.6
	golang.org/x/tools v0.6.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ProtonMail/go-crypto v0.0.0-20230217124315-7d5c6f04bbb8 // indirect
	github.com/andygrunwald/go-jira v1.15.1 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/containerd/cgroups/v3 v3.0.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/dghubble/go-twitter v0.0.0-20220626024101-68c0170dc641 // indirect
	github.com/dghubble/oauth1 v0.7.1 // indirect
	github.com/dghubble/sling v1.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/evergreen-ci/service v1.0.1-0.20200225230430-d9382e39d768 // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fuyufjh/splunk-hec-go v0.4.0 // indirect
	github.com/gen2brain/beeep v0.0.0-20220518085355-d7852edf42fc // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-git/go-billy/v5 v5.0.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.3.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mattn/go-xmpp v0.0.0-20220513082406-1411b9cc8b9a // indirect
	github.com/mholt/archiver v3.1.1+incompatible // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/phyber/negroni-gzip v1.0.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/cors v1.8.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/trivago/tgo v1.0.7 // indirect
	github.com/tychoish/birch/x/ftdc v0.0.0-20230824231239-7522c174b74b // indirect
	github.com/tychoish/birch/x/mrpc v0.0.0-20230815163402-467fbef7acab // indirect
	github.com/tychoish/gimlet v0.0.0-20230130001449-8987c96bb886 // indirect
	github.com/tychoish/grip/x/metrics v0.0.0-20240114232258-7eae5cf3a031 // indirect
	github.com/tychoish/grip/x/splunk v0.0.0-20240114232258-7eae5cf3a031 // indirect
	github.com/tychoish/jasper/x/remote v0.0.0-20230825020900-7d32edd66d81 // indirect
	github.com/tychoish/jasper/x/splunk v0.0.0-20230510193424-429e4caa8e98 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/urfave/negroni v1.0.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.1 // indirect
	github.com/xdg-go/stringprep v1.0.3 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/crypto v0.8.0 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/oauth2 v0.6.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
