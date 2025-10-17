package main

import (
	"flag"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"scutbot.cn/web/rmtv/internal/job"
	"scutbot.cn/web/rmtv/utils"
)

func main() {
	defaultResetTime := time.Now()
	defaultResetTime = defaultResetTime.Add(-time.Hour * 24)

	dbpath := flag.String("dbpath", "rmtv.db", "db path")
	setTime := flag.String("set-time", defaultResetTime.Format(time.DateTime), "set time cursor")
	flag.Parse()

	t, err := time.Parse(time.DateTime, *setTime)
	if err != nil {
		panic(err)
	}

	logrus.Infof("Resetting time cursor to %s", t.Format(time.DateTime))

	db, err := bbolt.Open(*dbpath, 0o600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(job.BucketName)
		if err != nil {
			return err
		}

		if err = bucket.Put(job.TimeCursorKey, utils.MarshalInt64(t.Unix())); err != nil {
			return errors.Wrapf(err, "failed to update time cursor")
		}

		return nil
	})
	if err != nil {
		panic(err)
	}

	logrus.Info("Done")
}
