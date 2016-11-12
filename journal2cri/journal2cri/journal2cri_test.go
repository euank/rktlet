package journal2cri

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/stretchr/testify/assert"
)

func TestProcessEntry(t *testing.T) {
	now := time.Unix(0xbad, 0xfacade)
	now = now.Round(time.Millisecond) // since the nanosecond part of now will break assertions
	timeInMillis := now.UnixNano() / 1000
	successPairs := []struct {
		In  sdjournal.JournalEntry
		Out CRIEntry
	}{
		{
			In: sdjournal.JournalEntry{
				RealtimeTimestamp: uint64(timeInMillis),
				Fields: map[string]string{
					"SYSLOG_IDENTIFIER": "myapp-1",
					"_TRANSPORT":        "stdout",
					"MESSAGE":           "20/20",
				},
			},
			Out: CRIEntry{
				AppName:    "myapp",
				AppAttempt: 1,
				Message:    "20/20",
				StreamType: CRIStreamStdout,
				Timestamp:  now,
			},
		},
		{
			In: sdjournal.JournalEntry{
				RealtimeTimestamp: uint64(timeInMillis),
				Fields: map[string]string{
					"SYSLOG_IDENTIFIER": "otherapp-10",
					"_TRANSPORT":        "stderr",
					"MESSAGE":           "petrov",
				},
			},
			Out: CRIEntry{
				AppName:    "otherapp",
				AppAttempt: 10,
				Message:    "petrov",
				StreamType: CRIStreamStderr,
				Timestamp:  now,
			},
		},
	}

	for i, pair := range successPairs {
		t.Run(fmt.Sprintf("%d success with %s", i, pair.Out.AppName), func(t *testing.T) {
			out := ProcessEntry(&pair.In)
			assert.Equal(t, &pair.Out, out)
		})
	}

}
