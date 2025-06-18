package utils

import (
	"github.com/sirupsen/logrus"
	"os"
)

var Debug = os.Getenv("DEBUG") == "true"

func init() {
	if Debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
