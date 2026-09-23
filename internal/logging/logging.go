// Package logging configures a logrus.Logger so that log.txt always
// receives every log entry (including Trace), while the console only
// shows entries at or above a configured level.
package logging

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// consoleHook writes formatted entries to writer, but only for levels
// at or above (i.e. no more verbose than) the configured level.
type consoleHook struct {
	writer    io.Writer
	formatter logrus.Formatter
	level     logrus.Level
}

func (h *consoleHook) Levels() []logrus.Level {
	return logrus.AllLevels[:h.level+1]
}

func (h *consoleHook) Fire(entry *logrus.Entry) error {
	line, err := h.formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = h.writer.Write(line)
	return err
}

// New creates a logrus.Logger that always writes every entry (up to Trace)
// to the file at logPath, while the console (stdout) only shows entries at
// consoleLevel or above. The returned close function must be called (e.g.
// via defer) to flush and close the log file.
func New(logPath string, consoleLevel logrus.Level, formatter logrus.Formatter) (*logrus.Logger, func(), error) {
	f, err := os.Create(logPath)
	if err != nil {
		return nil, nil, err
	}

	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(formatter)
	logger.SetOutput(f)
	logger.AddHook(&consoleHook{
		writer:    os.Stdout,
		formatter: formatter,
		level:     consoleLevel,
	})

	return logger, func() { f.Close() }, nil
}
