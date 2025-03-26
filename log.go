// contain logger presets for daemon

package main

import (
	"github.com/sirupsen/logrus"
	sHook "github.com/sirupsen/logrus/hooks/syslog"
	"log/syslog"
)

// SetupLogger return new logger for daemon. Read with journalctl -t fsyncd
func SetupLogger() (log *logrus.Logger, err error) {
	var hook *sHook.SyslogHook

	log = logrus.New()
	hook, err = sHook.NewSyslogHook("", "", syslog.LOG_DAEMON, "fsyncd")
	if err != nil {
		log.Hooks.Add(hook)
		return log, err
	}

	log.Error("syslog hook configuration failed")
	return nil, err
}
