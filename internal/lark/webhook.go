package lark

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

type WebhookProvider interface {
	GetWebhooks() ([]string, error)
}

type fileWebhookProvider struct {
	filePath string
}

func (f fileWebhookProvider) GetWebhooks() ([]string, error) {
	content, err := os.ReadFile(f.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("webhooks file at %s not set.", f.filePath)
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "failed to read file %s", f.filePath)
		}
	}

	return strings.Split(string(content), "\n"), nil
}

func NewFileWebhookProvider(filePath string) WebhookProvider {
	return &fileWebhookProvider{filePath: filePath}
}
