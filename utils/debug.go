package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Debug = os.Getenv("DEBUG") == "true"

func init() {
	if Debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
