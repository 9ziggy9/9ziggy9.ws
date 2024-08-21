package server

import (
	"runtime"
	"fmt"
	"log"
)

type LogLevel int
const (
	INFO = iota
	SUCCESS
	WARNING
	ERROR
)

var LogLevelStrMap = map[LogLevel]string{
	INFO:    "INFO",
	SUCCESS: "SUCCESS",
	WARNING: "WARNING",
	ERROR:   "ERROR",
}

func extractRuntimeMetaData() (string, string, int) {
	pc, file, line, ok := runtime.Caller(1)
	if !ok { panic("FATAL RUNTIME EXTRACTION ERROR") }
	fn := runtime.FuncForPC(pc).Name()
	return fn, file, line
}


type textColors struct {
	Reset		 string;
	Red			 string;
	Green		 string;
	Yellow	 string;
	Blue		 string;
	Magenta	 string;
	Cyan		 string;
	Gray		 string;
	White		 string;
}

var TEXT_COLORS = textColors{
	Reset   : "\033[0m" ,
	Red			: "\033[31m",
	Green		: "\033[32m",
	Yellow	: "\033[33m",
	Blue		: "\033[34m",
	Magenta	: "\033[35m",
	Cyan		: "\033[36m",
	Gray		: "\033[37m",
	White		: "\033[97m",
}

func ColorizeText(txt string, clr string) string {
	return clr + txt + TEXT_COLORS.Reset
}

const log_fmt_info string = "\n  -> [%s] %s\n";
const log_fmt_err  string = "\n  -> [%s] %s (@ %s :: %d)\n\n";

func Log(lvl LogLevel, msg string, optargs ...interface{}) {
	fmt.Printf("\n")
	switch (lvl) {
	case INFO:
		log.Printf(
			log_fmt_info,
			ColorizeText(LogLevelStrMap[lvl], TEXT_COLORS.Yellow),
			ColorizeText(fmt.Sprintf(msg, optargs...), TEXT_COLORS.Cyan),
		);
	case SUCCESS:
		log.Printf(
			log_fmt_info,
			ColorizeText(LogLevelStrMap[lvl], TEXT_COLORS.Green),
			ColorizeText(fmt.Sprintf(msg, optargs...), TEXT_COLORS.Cyan),
		);
	case ERROR:
		fn, _, line := extractRuntimeMetaData()
		log.Printf(
			log_fmt_err,
			ColorizeText(LogLevelStrMap[lvl], TEXT_COLORS.Red),
			ColorizeText(fmt.Sprintf(msg, optargs...), TEXT_COLORS.Cyan),
			fn, line,
		)
	}
}
