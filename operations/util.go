package operations

import (
	"os"
	"runtime"

	homedir "github.com/mitchellh/go-homedir"
)

func getHomeDir() string {
	userHome, err := homedir.Dir()
	if err != nil {
		// workaround for cygwin if we're on windows but couldn't get a homedir
		if runtime.GOOS == "windows" && len(os.Getenv("HOME")) > 0 {
			userHome = os.Getenv("HOME")
		}
	}

	return userHome
}
